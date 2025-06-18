package worker

import (
	"context"
	"errors"
	"sync"
	"sync/atomic"
	"time"
)

// ConnectionPool manages reusable connections for S3 operations
type ConnectionPool struct {
	// Connection management
	connections chan *PooledConnection
	factory     ConnectionFactory
	maxSize     int
	minSize     int
	currentSize int64

	// Health monitoring
	healthCheck   HealthChecker
	checkInterval time.Duration

	// Lifecycle management
	mu      sync.RWMutex
	closed  int64
	cleanup chan struct{}
	wg      sync.WaitGroup
}

// PooledConnection wraps a connection with metadata
type PooledConnection struct {
	Conn       interface{} // Actual connection
	CreatedAt  time.Time   // Creation timestamp
	LastUsed   time.Time   // Last usage timestamp
	UsageCount int64       // Number of times used
	IsHealthy  bool        // Health status
	ctx        context.Context
	cancel     context.CancelFunc
}

// ConnectionFactory creates new connections
type ConnectionFactory interface {
	CreateConnection(ctx context.Context) (interface{}, error)
	ValidateConnection(conn interface{}) error
	CloseConnection(conn interface{}) error
}

// HealthChecker validates connection health
type HealthChecker interface {
	IsHealthy(conn *PooledConnection) bool
	ShouldReplace(conn *PooledConnection) bool
}

// ConnectionPoolConfig configures the connection pool
type ConnectionPoolConfig struct {
	MinConnections int
	MaxConnections int
	Factory        ConnectionFactory
	HealthChecker  HealthChecker
	CheckInterval  time.Duration
	MaxIdleTime    time.Duration
	MaxLifetime    time.Duration
}

// NewConnectionPool creates a new high-performance connection pool
func NewConnectionPool(config *ConnectionPoolConfig) *ConnectionPool {
	if config == nil {
		config = &ConnectionPoolConfig{
			MinConnections: 2,
			MaxConnections: 20,
			CheckInterval:  time.Second * 30,
			MaxIdleTime:    time.Minute * 5,
			MaxLifetime:    time.Hour,
		}
	}

	pool := &ConnectionPool{
		connections:   make(chan *PooledConnection, config.MaxConnections),
		factory:       config.Factory,
		maxSize:       config.MaxConnections,
		minSize:       config.MinConnections,
		healthCheck:   config.HealthChecker,
		checkInterval: config.CheckInterval,
		cleanup:       make(chan struct{}),
	}

	return pool
}

// Start initializes the connection pool
func (cp *ConnectionPool) Start(ctx context.Context) error {
	cp.mu.Lock()
	defer cp.mu.Unlock()

	if atomic.LoadInt64(&cp.closed) == 1 {
		return ErrPoolClosed
	}

	// Pre-allocate minimum connections
	for i := 0; i < cp.minSize; i++ {
		conn, err := cp.createConnection(ctx)
		if err != nil {
			return err
		}
		cp.connections <- conn
	}

	// Start health monitoring
	cp.wg.Add(1)
	go cp.healthMonitor()

	return nil
}

// Get retrieves a connection from the pool
func (cp *ConnectionPool) Get(ctx context.Context) (*PooledConnection, error) {
	if atomic.LoadInt64(&cp.closed) == 1 {
		return nil, ErrPoolClosed
	}

	// Try to get from pool
	select {
	case conn := <-cp.connections:
		// Update usage statistics
		conn.LastUsed = time.Now()
		atomic.AddInt64(&conn.UsageCount, 1)
		return conn, nil
	default:
		// Pool is empty, create new connection if under limit
		if atomic.LoadInt64(&cp.currentSize) < int64(cp.maxSize) {
			return cp.createConnection(ctx)
		}

		// Wait for available connection with timeout
		select {
		case conn := <-cp.connections:
			conn.LastUsed = time.Now()
			atomic.AddInt64(&conn.UsageCount, 1)
			return conn, nil
		case <-ctx.Done():
			return nil, ctx.Err()
		}
	}
}

// Put returns a connection to the pool
func (cp *ConnectionPool) Put(conn *PooledConnection) error {
	if atomic.LoadInt64(&cp.closed) == 1 {
		return cp.closeConnection(conn)
	}

	// Validate connection health
	if cp.healthCheck != nil && !cp.healthCheck.IsHealthy(conn) {
		return cp.closeConnection(conn)
	}

	// Return to pool or close if pool is full
	select {
	case cp.connections <- conn:
		return nil
	default:
		// Pool is full, close connection
		return cp.closeConnection(conn)
	}
}

// Close gracefully shuts down the connection pool
func (cp *ConnectionPool) Close() error {
	atomic.StoreInt64(&cp.closed, 1)
	close(cp.cleanup)
	cp.wg.Wait()

	// Close all remaining connections
	close(cp.connections)
	for conn := range cp.connections {
		cp.closeConnection(conn)
	}

	return nil
}

// createConnection creates a new pooled connection
func (cp *ConnectionPool) createConnection(ctx context.Context) (*PooledConnection, error) {
	if cp.factory == nil {
		return nil, ErrNoConnectionFactory
	}

	conn, err := cp.factory.CreateConnection(ctx)
	if err != nil {
		return nil, err
	}

	pooledConn := &PooledConnection{
		Conn:       conn,
		CreatedAt:  time.Now(),
		LastUsed:   time.Now(),
		UsageCount: 0,
		IsHealthy:  true,
	}
	pooledConn.ctx, pooledConn.cancel = context.WithCancel(ctx)

	atomic.AddInt64(&cp.currentSize, 1)
	return pooledConn, nil
}

// closeConnection safely closes a pooled connection
func (cp *ConnectionPool) closeConnection(conn *PooledConnection) error {
	if conn.cancel != nil {
		conn.cancel()
	}

	var err error
	if cp.factory != nil {
		err = cp.factory.CloseConnection(conn.Conn)
	}

	atomic.AddInt64(&cp.currentSize, -1)
	return err
}

// healthMonitor continuously monitors connection health
func (cp *ConnectionPool) healthMonitor() {
	defer cp.wg.Done()

	ticker := time.NewTicker(cp.checkInterval)
	defer ticker.Stop()

	for {
		select {
		case <-cp.cleanup:
			return
		case <-ticker.C:
			cp.performHealthCheck()
		}
	}
}

// performHealthCheck validates and replaces unhealthy connections
func (cp *ConnectionPool) performHealthCheck() {
	if cp.healthCheck == nil {
		return
	}

	// Check connections in pool
	var healthyConns []*PooledConnection

	for {
		select {
		case conn := <-cp.connections:
			if cp.healthCheck.IsHealthy(conn) && !cp.healthCheck.ShouldReplace(conn) {
				healthyConns = append(healthyConns, conn)
			} else {
				cp.closeConnection(conn)
			}
		default:
			// No more connections to check
			goto restore
		}
	}

restore:
	// Return healthy connections to pool
	for _, conn := range healthyConns {
		select {
		case cp.connections <- conn:
		default:
			// Pool full, close excess connections
			cp.closeConnection(conn)
		}
	}

	// Ensure minimum connections
	current := atomic.LoadInt64(&cp.currentSize)
	for i := int(current); i < cp.minSize; i++ {
		conn, err := cp.createConnection(context.Background())
		if err != nil {
			break
		}

		select {
		case cp.connections <- conn:
		default:
			cp.closeConnection(conn)
			break
		}
	}
}

// GetStats returns connection pool statistics
func (cp *ConnectionPool) GetStats() *ConnectionPoolStats {
	return &ConnectionPoolStats{
		ActiveConnections: int(atomic.LoadInt64(&cp.currentSize)),
		IdleConnections:   len(cp.connections),
		MaxConnections:    cp.maxSize,
		MinConnections:    cp.minSize,
	}
}

// ConnectionPoolStats provides pool statistics
type ConnectionPoolStats struct {
	ActiveConnections int
	IdleConnections   int
	MaxConnections    int
	MinConnections    int
}

// Pool errors
var (
	ErrPoolClosed           = errors.New("connection pool is closed")
	ErrNoConnectionFactory  = errors.New("no connection factory provided")
	ErrConnectionValidation = errors.New("connection validation failed")
)
