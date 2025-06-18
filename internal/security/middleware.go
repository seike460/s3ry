package security

import (
	"fmt"
	"net/http"
	"strings"
	"time"
	// "github.com/seike460/s3ry/internal/security/enterprise"
)

// SecurityMiddleware provides security middleware for HTTP requests
type SecurityMiddleware struct {
	securityManager *SecurityManager
	config          *MiddlewareConfig
}

// MiddlewareConfig holds middleware configuration
type MiddlewareConfig struct {
	RequireAuth        bool     `json:"require_auth" yaml:"require_auth"`
	RequireMFA         bool     `json:"require_mfa" yaml:"require_mfa"`
	AllowedOrigins     []string `json:"allowed_origins" yaml:"allowed_origins"`
	RateLimitRequests  int      `json:"rate_limit_requests" yaml:"rate_limit_requests"`
	RateLimitWindow    int      `json:"rate_limit_window" yaml:"rate_limit_window"`
	EnableAuditLogging bool     `json:"enable_audit_logging" yaml:"enable_audit_logging"`
	CSPPolicy          string   `json:"csp_policy" yaml:"csp_policy"`
	EnableHSTS         bool     `json:"enable_hsts" yaml:"enable_hsts"`
}

// DefaultMiddlewareConfig returns default middleware configuration
func DefaultMiddlewareConfig() *MiddlewareConfig {
	return &MiddlewareConfig{
		RequireAuth:        true,
		RequireMFA:         false,
		AllowedOrigins:     []string{"https://localhost:*", "https://127.0.0.1:*"},
		RateLimitRequests:  100,
		RateLimitWindow:    60, // 60 seconds
		EnableAuditLogging: true,
		CSPPolicy:          "default-src 'self'; script-src 'self' 'unsafe-inline'",
		EnableHSTS:         true,
	}
}

// NewSecurityMiddleware creates a new security middleware
func NewSecurityMiddleware(securityManager *SecurityManager) *SecurityMiddleware {
	return &SecurityMiddleware{
		securityManager: securityManager,
		config:          DefaultMiddlewareConfig(),
	}
}

// AuthenticationMiddleware handles user authentication
func (sm *SecurityMiddleware) AuthenticationMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if !sm.config.RequireAuth || !sm.securityManager.IsSecurityEnabled() {
			next.ServeHTTP(w, r)
			return
		}

		// Extract authentication information
		authHeader := r.Header.Get("Authorization")
		if authHeader == "" {
			sm.writeErrorResponse(w, http.StatusUnauthorized, "Authentication required")
			return
		}

		// Parse Bearer token
		if !strings.HasPrefix(authHeader, "Bearer ") {
			sm.writeErrorResponse(w, http.StatusUnauthorized, "Invalid authentication format")
			return
		}

		token := strings.TrimPrefix(authHeader, "Bearer ")
		if token == "" {
			sm.writeErrorResponse(w, http.StatusUnauthorized, "Authentication token required")
			return
		}

		// Create authentication request
		// authReq := &enterprise.AuthenticationRequest{
		// UserID:    sm.extractUserIDFromToken(token),
		// SessionID: token,
		// IPAddress: sm.getClientIP(r),
		// UserAgent: r.UserAgent(),
		// Timestamp: time.Now(),
		// }

		// TODO: Authenticate user when authentication is implemented
		// result, err := sm.securityManager.AuthenticateUser(r.Context(), authReq)
		// if err != nil {
		//     sm.logSecurityEvent(r, "authentication_failed", map[string]interface{}{
		//         "error": err.Error(),
		//     })
		//     sm.writeErrorResponse(w, http.StatusUnauthorized, "Authentication failed")
		//     return
		// }

		// if !result.Authenticated {
		//     sm.logSecurityEvent(r, "authentication_denied", map[string]interface{}{
		//         "reason": result.Reason,
		//     })
		//     sm.writeErrorResponse(w, http.StatusUnauthorized, result.Reason)
		//     return
		// }

		// TODO: Check if MFA is required when authentication is implemented
		// if sm.config.RequireMFA && result.RequiresMFA {
		//     mfaToken := r.Header.Get("X-MFA-Token")
		//     if mfaToken == "" {
		//         sm.writeErrorResponse(w, http.StatusUnauthorized, "MFA token required")
		//         return
		//     }
		//     // MFA validation would be implemented here
		// }

		// TODO: Add user context to request when authentication is implemented
		// ctx := context.WithValue(r.Context(), "user_id", result.UserID)
		// ctx = context.WithValue(ctx, "session_id", result.Session.ID)
		// ctx = context.WithValue(ctx, "permissions", result.Permissions)

		// sm.logSecurityEvent(r, "authentication_success", map[string]interface{}{
		//     "user_id": result.UserID,
		// })

		// TODO: Pass user context when authentication is implemented
		next.ServeHTTP(w, r)
	})
}

// AuthorizationMiddleware handles user authorization
func (sm *SecurityMiddleware) AuthorizationMiddleware(requiredAction, requiredResource string) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			if !sm.securityManager.IsSecurityEnabled() {
				next.ServeHTTP(w, r)
				return
			}

			// Extract user information from context
			userID, ok := r.Context().Value("user_id").(string)
			if !ok {
				sm.writeErrorResponse(w, http.StatusUnauthorized, "User authentication required")
				return
			}

			sessionID, _ := r.Context().Value("session_id").(string)

			// Perform authorization
			err := sm.securityManager.AuthorizeAction(
				r.Context(),
				userID,
				requiredAction,
				requiredResource,
				sessionID,
				sm.getClientIP(r),
			)

			if err != nil {
				sm.logSecurityEvent(r, "authorization_failed", map[string]interface{}{
					"user_id":  userID,
					"action":   requiredAction,
					"resource": requiredResource,
					"error":    err.Error(),
				})
				sm.writeErrorResponse(w, http.StatusForbidden, "Access denied")
				return
			}

			sm.logSecurityEvent(r, "authorization_success", map[string]interface{}{
				"user_id":  userID,
				"action":   requiredAction,
				"resource": requiredResource,
			})

			next.ServeHTTP(w, r)
		})
	}
}

// SecurityHeadersMiddleware adds security headers to responses
func (sm *SecurityMiddleware) SecurityHeadersMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// Content Security Policy
		if sm.config.CSPPolicy != "" {
			w.Header().Set("Content-Security-Policy", sm.config.CSPPolicy)
		}

		// HSTS (HTTP Strict Transport Security)
		if sm.config.EnableHSTS {
			w.Header().Set("Strict-Transport-Security", "max-age=31536000; includeSubDomains")
		}

		// Other security headers
		w.Header().Set("X-Content-Type-Options", "nosniff")
		w.Header().Set("X-Frame-Options", "DENY")
		w.Header().Set("X-XSS-Protection", "1; mode=block")
		w.Header().Set("Referrer-Policy", "strict-origin-when-cross-origin")

		// Remove server identification
		w.Header().Set("Server", "")

		next.ServeHTTP(w, r)
	})
}

// CORSMiddleware handles Cross-Origin Resource Sharing
func (sm *SecurityMiddleware) CORSMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		origin := r.Header.Get("Origin")

		// Check if origin is allowed
		allowed := false
		for _, allowedOrigin := range sm.config.AllowedOrigins {
			if sm.matchOrigin(origin, allowedOrigin) {
				allowed = true
				break
			}
		}

		if allowed {
			w.Header().Set("Access-Control-Allow-Origin", origin)
			w.Header().Set("Access-Control-Allow-Methods", "GET, POST, PUT, DELETE, OPTIONS")
			w.Header().Set("Access-Control-Allow-Headers", "Authorization, Content-Type, X-MFA-Token")
			w.Header().Set("Access-Control-Allow-Credentials", "true")
		}

		// Handle preflight requests
		if r.Method == "OPTIONS" {
			w.WriteHeader(http.StatusOK)
			return
		}

		if !allowed && origin != "" {
			sm.logSecurityEvent(r, "cors_violation", map[string]interface{}{
				"origin": origin,
			})
			sm.writeErrorResponse(w, http.StatusForbidden, "Origin not allowed")
			return
		}

		next.ServeHTTP(w, r)
	})
}

// AuditMiddleware logs security-relevant events
func (sm *SecurityMiddleware) AuditMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if !sm.config.EnableAuditLogging || !sm.securityManager.IsSecurityEnabled() {
			next.ServeHTTP(w, r)
			return
		}

		start := time.Now()

		// Wrap response writer to capture status code
		wrapped := &responseWriter{ResponseWriter: w}

		next.ServeHTTP(wrapped, r)

		_ = time.Since(start) // TODO: Use duration when audit logging is implemented

		// Log the request
		userID, _ := r.Context().Value("user_id").(string)
		if userID == "" {
			userID = "anonymous"
		}

		auditLogger := sm.securityManager.GetAuditLogger()
		// TODO: Enable when audit logger interface is implemented
		// if auditLogger != nil {
		//     auditLogger.LogAccess(
		//         userID,
		//         fmt.Sprintf("session_%d", time.Now().Unix()),
		//         r.Method,
		//         r.URL.Path,
		//         fmt.Sprintf("HTTP_%d", wrapped.statusCode),
		//         sm.getClientIP(r),
		//         r.UserAgent(),
		//     )
		//
		//     // Log additional context
		//     auditLogger.LogAction(userID, "http_request", r.URL.Path, "COMPLETED", map[string]interface{}{
		//         "method":       r.Method,
		//         "status_code":  wrapped.statusCode,
		//         "duration_ms":  duration.Milliseconds(),
		//         "content_length": r.ContentLength,
		//         "referer":      r.Referer(),
		//     })
		// }
		_ = auditLogger // Suppress unused variable warning
	})
}

// RateLimitMiddleware implements rate limiting
func (sm *SecurityMiddleware) RateLimitMiddleware(next http.Handler) http.Handler {
	// Simple in-memory rate limiter (production would use Redis or similar)
	clients := make(map[string]*rateLimitInfo)

	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if sm.config.RateLimitRequests == 0 {
			next.ServeHTTP(w, r)
			return
		}

		clientIP := sm.getClientIP(r)
		now := time.Now()

		// Clean up old entries
		for ip, info := range clients {
			if now.Sub(info.windowStart) > time.Duration(sm.config.RateLimitWindow)*time.Second {
				delete(clients, ip)
			}
		}

		// Check rate limit
		info, exists := clients[clientIP]
		if !exists {
			clients[clientIP] = &rateLimitInfo{
				requests:    1,
				windowStart: now,
			}
		} else {
			if now.Sub(info.windowStart) > time.Duration(sm.config.RateLimitWindow)*time.Second {
				// Reset window
				info.requests = 1
				info.windowStart = now
			} else {
				info.requests++
				if info.requests > sm.config.RateLimitRequests {
					sm.logSecurityEvent(r, "rate_limit_exceeded", map[string]interface{}{
						"client_ip": clientIP,
						"requests":  info.requests,
					})
					sm.writeErrorResponse(w, http.StatusTooManyRequests, "Rate limit exceeded")
					return
				}
			}
		}

		next.ServeHTTP(w, r)
	})
}

// Helper types and functions

type responseWriter struct {
	http.ResponseWriter
	statusCode int
}

func (rw *responseWriter) WriteHeader(code int) {
	rw.statusCode = code
	rw.ResponseWriter.WriteHeader(code)
}

type rateLimitInfo struct {
	requests    int
	windowStart time.Time
}

// getClientIP extracts the client IP address from the request
func (sm *SecurityMiddleware) getClientIP(r *http.Request) string {
	// Check X-Forwarded-For header first
	forwarded := r.Header.Get("X-Forwarded-For")
	if forwarded != "" {
		ips := strings.Split(forwarded, ",")
		return strings.TrimSpace(ips[0])
	}

	// Check X-Real-IP header
	realIP := r.Header.Get("X-Real-IP")
	if realIP != "" {
		return realIP
	}

	// Fall back to RemoteAddr
	addr := r.RemoteAddr
	if idx := strings.LastIndex(addr, ":"); idx != -1 {
		return addr[:idx]
	}
	return addr
}

// extractUserIDFromToken extracts user ID from authentication token
func (sm *SecurityMiddleware) extractUserIDFromToken(token string) string {
	// This is a simplified implementation
	// In production, you would decode a JWT or validate against a session store
	if strings.HasPrefix(token, "user_") {
		return strings.TrimPrefix(token, "user_")
	}
	return "unknown"
}

// matchOrigin checks if an origin matches an allowed origin pattern
func (sm *SecurityMiddleware) matchOrigin(origin, pattern string) bool {
	if pattern == "*" {
		return true
	}
	if strings.HasSuffix(pattern, "*") {
		prefix := strings.TrimSuffix(pattern, "*")
		return strings.HasPrefix(origin, prefix)
	}
	return origin == pattern
}

// writeErrorResponse writes a standardized error response
func (sm *SecurityMiddleware) writeErrorResponse(w http.ResponseWriter, statusCode int, message string) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(statusCode)

	errorResponse := fmt.Sprintf(`{
		"error": {
			"code": %d,
			"message": "%s",
			"timestamp": "%s"
		}
	}`, statusCode, message, time.Now().UTC().Format(time.RFC3339))

	w.Write([]byte(errorResponse))
}

// logSecurityEvent logs a security event
func (sm *SecurityMiddleware) logSecurityEvent(r *http.Request, event string, context map[string]interface{}) {
	if !sm.securityManager.IsSecurityEnabled() {
		return
	}

	auditLogger := sm.securityManager.GetAuditLogger()
	if auditLogger != nil {
		userID, _ := r.Context().Value("user_id").(string)
		if userID == "" {
			userID = "anonymous"
		}

		// Add request context
		if context == nil {
			context = make(map[string]interface{})
		}
		context["ip_address"] = sm.getClientIP(r)
		context["user_agent"] = r.UserAgent()
		context["url"] = r.URL.Path
		context["method"] = r.Method

		// auditLogger.LogSecurityEvent(enterprise.AuditLevelInfo, userID, event, fmt.Sprintf("Security event: %s", event))
	}
}

// SetConfig updates the middleware configuration
func (sm *SecurityMiddleware) SetConfig(config *MiddlewareConfig) {
	sm.config = config
}
