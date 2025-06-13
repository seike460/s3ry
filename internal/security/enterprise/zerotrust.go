package enterprise

import (
	"context"
	"crypto/tls"
	"crypto/x509"
	"fmt"
	"net"
	"strings"
	"sync"
	"time"
)

// ZeroTrustConfig holds zero trust configuration
type ZeroTrustConfig struct {
	Enabled                 bool          `json:"enabled"`
	RequireMutualTLS        bool          `json:"require_mutual_tls"`
	VerifyPeerCertificates  bool          `json:"verify_peer_certificates"`
	AllowedCertificates     []string      `json:"allowed_certificates"`      // Certificate fingerprints
	RequiredCertificateOUs  []string      `json:"required_certificate_ous"`  // Required Organizational Units
	NetworkPolicyEnabled    bool          `json:"network_policy_enabled"`
	AllowedNetworks         []string      `json:"allowed_networks"`          // CIDR blocks
	DeniedNetworks          []string      `json:"denied_networks"`           // CIDR blocks
	SessionTimeout          time.Duration `json:"session_timeout"`
	RequireReauthentication bool          `json:"require_reauthentication"`
	MinimumTLSVersion       string        `json:"minimum_tls_version"`       // TLS 1.2, TLS 1.3
	CipherSuites            []string      `json:"cipher_suites"`
}

// DefaultZeroTrustConfig returns default zero trust configuration
func DefaultZeroTrustConfig() *ZeroTrustConfig {
	return &ZeroTrustConfig{
		Enabled:                 true,
		RequireMutualTLS:        true,
		VerifyPeerCertificates:  true,
		AllowedCertificates:     []string{},
		RequiredCertificateOUs:  []string{},
		NetworkPolicyEnabled:    true,
		AllowedNetworks:         []string{"10.0.0.0/8", "172.16.0.0/12", "192.168.0.0/16"},
		DeniedNetworks:          []string{},
		SessionTimeout:          time.Hour * 8,
		RequireReauthentication: true,
		MinimumTLSVersion:       "TLS 1.2",
		CipherSuites: []string{
			"TLS_ECDHE_RSA_WITH_AES_256_GCM_SHA384",
			"TLS_ECDHE_ECDSA_WITH_AES_256_GCM_SHA384",
			"TLS_ECDHE_RSA_WITH_AES_128_GCM_SHA256",
			"TLS_ECDHE_ECDSA_WITH_AES_128_GCM_SHA256",
		},
	}
}

// ZeroTrustManager manages zero trust security policies
type ZeroTrustManager struct {
	config         *ZeroTrustConfig
	networkPolicy  *NetworkPolicy
	sessionManager *SessionManager
	mutex          sync.RWMutex
}

// NewZeroTrustManager creates a new zero trust manager
func NewZeroTrustManager(config *ZeroTrustConfig) (*ZeroTrustManager, error) {
	if config == nil {
		config = DefaultZeroTrustConfig()
	}

	networkPolicy, err := NewNetworkPolicy(config.AllowedNetworks, config.DeniedNetworks)
	if err != nil {
		return nil, fmt.Errorf("failed to create network policy: %w", err)
	}

	sessionManager := NewSessionManager(config.SessionTimeout)

	return &ZeroTrustManager{
		config:         config,
		networkPolicy:  networkPolicy,
		sessionManager: sessionManager,
	}, nil
}

// ValidateConnection validates a connection according to zero trust principles
func (z *ZeroTrustManager) ValidateConnection(conn net.Conn, userID string) error {
	if !z.config.Enabled {
		return nil
	}

	z.mutex.RLock()
	defer z.mutex.RUnlock()

	// Get remote address
	remoteAddr := conn.RemoteAddr()
	if remoteAddr == nil {
		return fmt.Errorf("unable to determine remote address")
	}

	// Extract IP address
	var ip net.IP
	switch addr := remoteAddr.(type) {
	case *net.TCPAddr:
		ip = addr.IP
	case *net.UDPAddr:
		ip = addr.IP
	default:
		// Try to parse the address string
		host, _, err := net.SplitHostPort(remoteAddr.String())
		if err != nil {
			return fmt.Errorf("unable to parse remote address: %w", err)
		}
		ip = net.ParseIP(host)
		if ip == nil {
			return fmt.Errorf("invalid IP address: %s", host)
		}
	}

	// Validate network policy
	if z.config.NetworkPolicyEnabled {
		if !z.networkPolicy.IsAllowed(ip) {
			return fmt.Errorf("connection from %s denied by network policy", ip.String())
		}
	}

	// Validate TLS connection
	if tlsConn, ok := conn.(*tls.Conn); ok {
		if err := z.validateTLSConnection(tlsConn); err != nil {
			return fmt.Errorf("TLS validation failed: %w", err)
		}
	} else if z.config.RequireMutualTLS {
		return fmt.Errorf("TLS connection required but not provided")
	}

	// Validate session
	if userID != "" {
		if err := z.sessionManager.ValidateSession(userID); err != nil {
			return fmt.Errorf("session validation failed: %w", err)
		}
	}

	return nil
}

// validateTLSConnection validates TLS connection parameters
func (z *ZeroTrustManager) validateTLSConnection(tlsConn *tls.Conn) error {
	state := tlsConn.ConnectionState()

	// Check TLS version
	minVersion := z.getTLSVersion(z.config.MinimumTLSVersion)
	if state.Version < minVersion {
		return fmt.Errorf("TLS version %d below minimum %d", state.Version, minVersion)
	}

	// Verify peer certificates if required
	if z.config.VerifyPeerCertificates && len(state.PeerCertificates) == 0 {
		return fmt.Errorf("peer certificate required but not provided")
	}

	// Check certificate constraints
	if len(state.PeerCertificates) > 0 {
		cert := state.PeerCertificates[0]

		// Check allowed certificates
		if len(z.config.AllowedCertificates) > 0 {
			fingerprint := fmt.Sprintf("%x", cert.Signature)
			allowed := false
			for _, allowedFingerprint := range z.config.AllowedCertificates {
				if fingerprint == allowedFingerprint {
					allowed = true
					break
				}
			}
			if !allowed {
				return fmt.Errorf("certificate fingerprint not in allowed list")
			}
		}

		// Check required OUs
		if len(z.config.RequiredCertificateOUs) > 0 {
			hasRequiredOU := false
			for _, ou := range cert.Subject.OrganizationalUnit {
				for _, requiredOU := range z.config.RequiredCertificateOUs {
					if ou == requiredOU {
						hasRequiredOU = true
						break
					}
				}
				if hasRequiredOU {
					break
				}
			}
			if !hasRequiredOU {
				return fmt.Errorf("certificate does not contain required organizational unit")
			}
		}
	}

	return nil
}

// getTLSVersion converts string TLS version to constant
func (z *ZeroTrustManager) getTLSVersion(version string) uint16 {
	switch strings.ToUpper(version) {
	case "TLS 1.0":
		return tls.VersionTLS10
	case "TLS 1.1":
		return tls.VersionTLS11
	case "TLS 1.2":
		return tls.VersionTLS12
	case "TLS 1.3":
		return tls.VersionTLS13
	default:
		return tls.VersionTLS12 // Default to TLS 1.2
	}
}

// NetworkPolicy manages network access policies
type NetworkPolicy struct {
	allowedNetworks []*net.IPNet
	deniedNetworks  []*net.IPNet
}

// NewNetworkPolicy creates a new network policy
func NewNetworkPolicy(allowedCIDRs, deniedCIDRs []string) (*NetworkPolicy, error) {
	policy := &NetworkPolicy{}

	// Parse allowed networks
	for _, cidr := range allowedCIDRs {
		_, network, err := net.ParseCIDR(cidr)
		if err != nil {
			return nil, fmt.Errorf("invalid allowed network CIDR %s: %w", cidr, err)
		}
		policy.allowedNetworks = append(policy.allowedNetworks, network)
	}

	// Parse denied networks
	for _, cidr := range deniedCIDRs {
		_, network, err := net.ParseCIDR(cidr)
		if err != nil {
			return nil, fmt.Errorf("invalid denied network CIDR %s: %w", cidr, err)
		}
		policy.deniedNetworks = append(policy.deniedNetworks, network)
	}

	return policy, nil
}

// IsAllowed checks if an IP address is allowed by the network policy
func (n *NetworkPolicy) IsAllowed(ip net.IP) bool {
	// Check if IP is in denied networks first
	for _, network := range n.deniedNetworks {
		if network.Contains(ip) {
			return false
		}
	}

	// If no allowed networks specified, allow all (except denied)
	if len(n.allowedNetworks) == 0 {
		return true
	}

	// Check if IP is in allowed networks
	for _, network := range n.allowedNetworks {
		if network.Contains(ip) {
			return true
		}
	}

	return false
}

// SessionManager manages user sessions
type SessionManager struct {
	sessions      map[string]*Session
	sessionTimeout time.Duration
	mutex         sync.RWMutex
}

// Session represents a user session
type Session struct {
	UserID      string    `json:"user_id"`
	CreatedAt   time.Time `json:"created_at"`
	LastAccess  time.Time `json:"last_access"`
	IPAddress   string    `json:"ip_address"`
	UserAgent   string    `json:"user_agent"`
	Authenticated bool    `json:"authenticated"`
}

// NewSessionManager creates a new session manager
func NewSessionManager(timeout time.Duration) *SessionManager {
	sm := &SessionManager{
		sessions:       make(map[string]*Session),
		sessionTimeout: timeout,
	}

	// Start cleanup goroutine
	go sm.cleanupExpiredSessions()

	return sm
}

// CreateSession creates a new user session
func (s *SessionManager) CreateSession(userID, ipAddress, userAgent string) *Session {
	s.mutex.Lock()
	defer s.mutex.Unlock()

	session := &Session{
		UserID:        userID,
		CreatedAt:     time.Now(),
		LastAccess:    time.Now(),
		IPAddress:     ipAddress,
		UserAgent:     userAgent,
		Authenticated: true,
	}

	s.sessions[userID] = session
	return session
}

// ValidateSession validates a user session
func (s *SessionManager) ValidateSession(userID string) error {
	s.mutex.RLock()
	defer s.mutex.RUnlock()

	session, exists := s.sessions[userID]
	if !exists {
		return fmt.Errorf("session not found for user %s", userID)
	}

	if !session.Authenticated {
		return fmt.Errorf("session not authenticated for user %s", userID)
	}

	// Check if session has expired
	if time.Since(session.LastAccess) > s.sessionTimeout {
		return fmt.Errorf("session expired for user %s", userID)
	}

	// Update last access time
	session.LastAccess = time.Now()

	return nil
}

// InvalidateSession invalidates a user session
func (s *SessionManager) InvalidateSession(userID string) {
	s.mutex.Lock()
	defer s.mutex.Unlock()

	delete(s.sessions, userID)
}

// cleanupExpiredSessions periodically removes expired sessions
func (s *SessionManager) cleanupExpiredSessions() {
	ticker := time.NewTicker(time.Minute * 5)
	defer ticker.Stop()

	for range ticker.C {
		s.mutex.Lock()
		now := time.Now()
		for userID, session := range s.sessions {
			if now.Sub(session.LastAccess) > s.sessionTimeout {
				delete(s.sessions, userID)
			}
		}
		s.mutex.Unlock()
	}
}

// ZeroTrustContext provides zero trust context for operations
type ZeroTrustContext struct {
	UserID      string
	IPAddress   string
	UserAgent   string
	RequestID   string
	Permissions []Permission
}

// ZeroTrustMiddleware creates middleware for zero trust validation
func (z *ZeroTrustManager) ZeroTrustMiddleware() func(context.Context, *ZeroTrustContext) error {
	return func(ctx context.Context, ztCtx *ZeroTrustContext) error {
		if !z.config.Enabled {
			return nil
		}

		// Parse IP address
		ip := net.ParseIP(ztCtx.IPAddress)
		if ip == nil {
			return fmt.Errorf("invalid IP address: %s", ztCtx.IPAddress)
		}

		// Validate network policy
		if z.config.NetworkPolicyEnabled {
			if !z.networkPolicy.IsAllowed(ip) {
				return fmt.Errorf("access denied by network policy for IP %s", ztCtx.IPAddress)
			}
		}

		// Validate session
		if ztCtx.UserID != "" {
			if err := z.sessionManager.ValidateSession(ztCtx.UserID); err != nil {
				return fmt.Errorf("session validation failed: %w", err)
			}
		}

		return nil
	}
}

// CreateTLSConfig creates a TLS configuration based on zero trust settings
func (z *ZeroTrustManager) CreateTLSConfig() *tls.Config {
	config := &tls.Config{
		MinVersion: z.getTLSVersion(z.config.MinimumTLSVersion),
	}

	if z.config.RequireMutualTLS {
		config.ClientAuth = tls.RequireAndVerifyClientCert
	}

	if z.config.VerifyPeerCertificates {
		config.VerifyPeerCertificate = z.verifyPeerCertificate
	}

	return config
}

// verifyPeerCertificate custom peer certificate verification
func (z *ZeroTrustManager) verifyPeerCertificate(rawCerts [][]byte, verifiedChains [][]*x509.Certificate) error {
	// Custom certificate verification logic
	// This would implement additional checks beyond standard verification
	return nil
}