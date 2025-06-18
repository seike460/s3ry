package enterprise

import (
	"crypto/rand"
	"crypto/rsa"
	"crypto/sha256"
	"crypto/tls"
	"crypto/x509"
	"crypto/x509/pkix"
	"encoding/hex"
	"encoding/pem"
	"fmt"
	"math/big"
	"net"
	"sync"
	"time"
)

// CertificateManager manages SSL/TLS certificates and validation
type CertificateManager struct {
	config              *CertificateConfig
	trustedCertificates map[string]*x509.Certificate
	certificateCache    map[string]*CachedCertificate
	revocationList      map[string]time.Time
	pinnedCertificates  map[string]string // hostname -> fingerprint
	mutex               sync.RWMutex
}

// CertificateConfig holds certificate management configuration
type CertificateConfig struct {
	EnableCertPinning    bool          `json:"enable_cert_pinning"`
	ValidateChain        bool          `json:"validate_chain"`
	CheckRevocation      bool          `json:"check_revocation"`
	MaxCertAge           time.Duration `json:"max_cert_age"`
	RequireValidHostname bool          `json:"require_valid_hostname"`
	AllowSelfSigned      bool          `json:"allow_self_signed"`
	MinKeySize           int           `json:"min_key_size"`
	RequiredCipherSuites []uint16      `json:"required_cipher_suites"`
	MinTLSVersion        uint16        `json:"min_tls_version"`
	CacheExpirationHours int           `json:"cache_expiration_hours"`
}

// CachedCertificate holds cached certificate information
type CachedCertificate struct {
	Certificate *x509.Certificate `json:"certificate"`
	Fingerprint string            `json:"fingerprint"`
	ValidUntil  time.Time         `json:"valid_until"`
	CachedAt    time.Time         `json:"cached_at"`
	Hostname    string            `json:"hostname"`
	TrustLevel  TrustLevel        `json:"trust_level"`
}

// TrustLevel represents certificate trust levels
type TrustLevel int

const (
	TrustLevelUntrusted TrustLevel = iota
	TrustLevelLow
	TrustLevelMedium
	TrustLevelHigh
	TrustLevelPinned
)

// DefaultCertificateConfig returns default certificate configuration
func DefaultCertificateConfig() *CertificateConfig {
	return &CertificateConfig{
		EnableCertPinning:    true,
		ValidateChain:        true,
		CheckRevocation:      true,
		MaxCertAge:           time.Hour * 24 * 90, // 90 days
		RequireValidHostname: true,
		AllowSelfSigned:      false,
		MinKeySize:           2048,
		RequiredCipherSuites: []uint16{
			tls.TLS_ECDHE_RSA_WITH_AES_256_GCM_SHA384,
			tls.TLS_ECDHE_RSA_WITH_AES_128_GCM_SHA256,
			tls.TLS_ECDHE_ECDSA_WITH_AES_256_GCM_SHA384,
			tls.TLS_ECDHE_ECDSA_WITH_AES_128_GCM_SHA256,
		},
		MinTLSVersion:        tls.VersionTLS12,
		CacheExpirationHours: 24,
	}
}

// NewCertificateManager creates a new certificate manager
func NewCertificateManager(config *CertificateConfig) *CertificateManager {
	if config == nil {
		config = DefaultCertificateConfig()
	}

	cm := &CertificateManager{
		config:              config,
		trustedCertificates: make(map[string]*x509.Certificate),
		certificateCache:    make(map[string]*CachedCertificate),
		revocationList:      make(map[string]time.Time),
		pinnedCertificates:  make(map[string]string),
	}

	// Initialize with common pinned certificates (AWS, Google, etc.)
	cm.initializeDefaultPins()

	return cm
}

// ValidateCertificate performs comprehensive certificate validation
func (cm *CertificateManager) ValidateCertificate(cert *x509.Certificate, hostname string) (*CertificateValidationResult, error) {
	cm.mutex.RLock()
	defer cm.mutex.RUnlock()

	result := &CertificateValidationResult{
		Valid:       true,
		TrustLevel:  TrustLevelLow,
		Issues:      []string{},
		Warnings:    []string{},
		Fingerprint: cm.calculateFingerprint(cert),
		Hostname:    hostname,
	}

	// 1. Check certificate expiration
	if time.Now().After(cert.NotAfter) {
		result.Valid = false
		result.Issues = append(result.Issues, "Certificate has expired")
	}

	if time.Now().Before(cert.NotBefore) {
		result.Valid = false
		result.Issues = append(result.Issues, "Certificate is not yet valid")
	}

	// 2. Check certificate age
	age := time.Since(cert.NotBefore)
	if age > cm.config.MaxCertAge {
		result.Warnings = append(result.Warnings, fmt.Sprintf("Certificate is older than recommended maximum age of %v", cm.config.MaxCertAge))
	}

	// 3. Validate hostname if required
	if cm.config.RequireValidHostname && hostname != "" {
		if err := cert.VerifyHostname(hostname); err != nil {
			result.Valid = false
			result.Issues = append(result.Issues, fmt.Sprintf("Hostname verification failed: %v", err))
		}
	}

	// 4. Check key size
	if rsaKey, ok := cert.PublicKey.(*rsa.PublicKey); ok {
		if rsaKey.N.BitLen() < cm.config.MinKeySize {
			result.Valid = false
			result.Issues = append(result.Issues, fmt.Sprintf("RSA key size %d is below minimum %d", rsaKey.N.BitLen(), cm.config.MinKeySize))
		}
	}

	// 5. Check if certificate is self-signed
	if cert.Issuer.String() == cert.Subject.String() {
		if !cm.config.AllowSelfSigned {
			result.Valid = false
			result.Issues = append(result.Issues, "Self-signed certificates are not allowed")
		} else {
			result.Warnings = append(result.Warnings, "Certificate is self-signed")
			result.TrustLevel = TrustLevelUntrusted
		}
	}

	// 6. Check revocation status
	if cm.config.CheckRevocation {
		if revocationTime, revoked := cm.revocationList[result.Fingerprint]; revoked {
			result.Valid = false
			result.Issues = append(result.Issues, fmt.Sprintf("Certificate was revoked at %v", revocationTime))
		}
	}

	// 7. Check certificate pinning
	if cm.config.EnableCertPinning && hostname != "" {
		if pinnedFingerprint, pinned := cm.pinnedCertificates[hostname]; pinned {
			if pinnedFingerprint == result.Fingerprint {
				result.TrustLevel = TrustLevelPinned
			} else {
				result.Valid = false
				result.Issues = append(result.Issues, "Certificate does not match pinned certificate")
			}
		}
	}

	// 8. Determine final trust level
	if result.Valid {
		if result.TrustLevel != TrustLevelPinned {
			if len(result.Warnings) == 0 {
				result.TrustLevel = TrustLevelHigh
			} else if len(result.Warnings) <= 2 {
				result.TrustLevel = TrustLevelMedium
			} else {
				result.TrustLevel = TrustLevelLow
			}
		}
	} else {
		result.TrustLevel = TrustLevelUntrusted
	}

	// Cache the result
	cm.cacheValidationResult(cert, hostname, result)

	return result, nil
}

// CertificateValidationResult holds the result of certificate validation
type CertificateValidationResult struct {
	Valid       bool       `json:"valid"`
	TrustLevel  TrustLevel `json:"trust_level"`
	Issues      []string   `json:"issues"`
	Warnings    []string   `json:"warnings"`
	Fingerprint string     `json:"fingerprint"`
	Hostname    string     `json:"hostname"`
	ValidFrom   time.Time  `json:"valid_from"`
	ValidUntil  time.Time  `json:"valid_until"`
}

// ValidateConnectionSecurity validates the security of a TLS connection
func (cm *CertificateManager) ValidateConnectionSecurity(conn *tls.Conn) (*ConnectionSecurityResult, error) {
	state := conn.ConnectionState()

	result := &ConnectionSecurityResult{
		Secure:           true,
		TLSVersion:       state.Version,
		CipherSuite:      state.CipherSuite,
		PeerCertificates: state.PeerCertificates,
		Issues:           []string{},
		Warnings:         []string{},
	}

	// 1. Check TLS version
	if state.Version < cm.config.MinTLSVersion {
		result.Secure = false
		result.Issues = append(result.Issues, fmt.Sprintf("TLS version %x is below minimum %x", state.Version, cm.config.MinTLSVersion))
	}

	// 2. Check cipher suite
	if len(cm.config.RequiredCipherSuites) > 0 {
		cipherAllowed := false
		for _, allowedCipher := range cm.config.RequiredCipherSuites {
			if state.CipherSuite == allowedCipher {
				cipherAllowed = true
				break
			}
		}
		if !cipherAllowed {
			result.Secure = false
			result.Issues = append(result.Issues, fmt.Sprintf("Cipher suite %x is not in the allowed list", state.CipherSuite))
		}
	}

	// 3. Validate peer certificates
	if len(state.PeerCertificates) > 0 {
		hostname := conn.RemoteAddr().String()
		if host, _, err := net.SplitHostPort(hostname); err == nil {
			hostname = host
		}

		certResult, err := cm.ValidateCertificate(state.PeerCertificates[0], hostname)
		if err != nil {
			result.Issues = append(result.Issues, fmt.Sprintf("Certificate validation error: %v", err))
			result.Secure = false
		} else {
			result.CertificateResult = certResult
			if !certResult.Valid {
				result.Secure = false
				result.Issues = append(result.Issues, "Peer certificate validation failed")
			}
		}
	}

	return result, nil
}

// ConnectionSecurityResult holds the result of connection security validation
type ConnectionSecurityResult struct {
	Secure            bool                         `json:"secure"`
	TLSVersion        uint16                       `json:"tls_version"`
	CipherSuite       uint16                       `json:"cipher_suite"`
	PeerCertificates  []*x509.Certificate          `json:"peer_certificates"`
	CertificateResult *CertificateValidationResult `json:"certificate_result,omitempty"`
	Issues            []string                     `json:"issues"`
	Warnings          []string                     `json:"warnings"`
}

// PinCertificate pins a certificate for a specific hostname
func (cm *CertificateManager) PinCertificate(hostname string, cert *x509.Certificate) {
	cm.mutex.Lock()
	defer cm.mutex.Unlock()

	fingerprint := cm.calculateFingerprint(cert)
	cm.pinnedCertificates[hostname] = fingerprint
	cm.trustedCertificates[fingerprint] = cert
}

// RevokeCertificate adds a certificate to the revocation list
func (cm *CertificateManager) RevokeCertificate(cert *x509.Certificate) {
	cm.mutex.Lock()
	defer cm.mutex.Unlock()

	fingerprint := cm.calculateFingerprint(cert)
	cm.revocationList[fingerprint] = time.Now()
}

// GenerateSelfSignedCertificate generates a self-signed certificate for testing
func (cm *CertificateManager) GenerateSelfSignedCertificate(hostname string) (*tls.Certificate, error) {
	// Generate private key
	priv, err := rsa.GenerateKey(rand.Reader, 2048)
	if err != nil {
		return nil, fmt.Errorf("failed to generate private key: %w", err)
	}

	// Create certificate template
	template := x509.Certificate{
		SerialNumber: big.NewInt(1),
		Subject: pkix.Name{
			Organization:  []string{"S3ry Test"},
			Country:       []string{"US"},
			Province:      []string{""},
			Locality:      []string{""},
			StreetAddress: []string{""},
			PostalCode:    []string{""},
		},
		NotBefore:   time.Now(),
		NotAfter:    time.Now().Add(365 * 24 * time.Hour), // Valid for 1 year
		KeyUsage:    x509.KeyUsageKeyEncipherment | x509.KeyUsageDigitalSignature,
		ExtKeyUsage: []x509.ExtKeyUsage{x509.ExtKeyUsageServerAuth},
		IPAddresses: []net.IP{net.IPv4(127, 0, 0, 1), net.IPv6loopback},
	}

	if hostname != "" {
		template.DNSNames = []string{hostname}
	}

	// Generate certificate
	certDER, err := x509.CreateCertificate(rand.Reader, &template, &template, &priv.PublicKey, priv)
	if err != nil {
		return nil, fmt.Errorf("failed to create certificate: %w", err)
	}

	// Create TLS certificate
	tlsCert := &tls.Certificate{
		Certificate: [][]byte{certDER},
		PrivateKey:  priv,
	}

	return tlsCert, nil
}

// calculateFingerprint calculates SHA-256 fingerprint of a certificate
func (cm *CertificateManager) calculateFingerprint(cert *x509.Certificate) string {
	hash := sha256.Sum256(cert.Raw)
	return hex.EncodeToString(hash[:])
}

// cacheValidationResult caches certificate validation results
func (cm *CertificateManager) cacheValidationResult(cert *x509.Certificate, hostname string, result *CertificateValidationResult) {
	cm.mutex.Lock()
	defer cm.mutex.Unlock()

	cached := &CachedCertificate{
		Certificate: cert,
		Fingerprint: result.Fingerprint,
		ValidUntil:  cert.NotAfter,
		CachedAt:    time.Now(),
		Hostname:    hostname,
		TrustLevel:  result.TrustLevel,
	}

	cacheKey := fmt.Sprintf("%s:%s", hostname, result.Fingerprint)
	cm.certificateCache[cacheKey] = cached
}

// getCachedValidationResult retrieves cached validation results
func (cm *CertificateManager) getCachedValidationResult(cert *x509.Certificate, hostname string) *CachedCertificate {
	cm.mutex.RLock()
	defer cm.mutex.RUnlock()

	fingerprint := cm.calculateFingerprint(cert)
	cacheKey := fmt.Sprintf("%s:%s", hostname, fingerprint)

	if cached, exists := cm.certificateCache[cacheKey]; exists {
		// Check if cache is still valid
		expirationTime := cached.CachedAt.Add(time.Duration(cm.config.CacheExpirationHours) * time.Hour)
		if time.Now().Before(expirationTime) {
			return cached
		}
		// Remove expired cache entry
		delete(cm.certificateCache, cacheKey)
	}

	return nil
}

// initializeDefaultPins sets up default certificate pins for common services
func (cm *CertificateManager) initializeDefaultPins() {
	// AWS S3 endpoints (these would be actual fingerprints in production)
	defaultPins := map[string]string{
		"s3.amazonaws.com":                "example_fingerprint_aws_s3",
		"s3.us-east-1.amazonaws.com":      "example_fingerprint_aws_s3_us_east_1",
		"s3.us-west-2.amazonaws.com":      "example_fingerprint_aws_s3_us_west_2",
		"s3.eu-west-1.amazonaws.com":      "example_fingerprint_aws_s3_eu_west_1",
		"s3.ap-northeast-1.amazonaws.com": "example_fingerprint_aws_s3_ap_northeast_1",
	}

	for hostname, fingerprint := range defaultPins {
		cm.pinnedCertificates[hostname] = fingerprint
	}
}

// ExportCertificatePEM exports a certificate in PEM format
func (cm *CertificateManager) ExportCertificatePEM(cert *x509.Certificate) (string, error) {
	block := &pem.Block{
		Type:  "CERTIFICATE",
		Bytes: cert.Raw,
	}
	return string(pem.EncodeToMemory(block)), nil
}

// ImportCertificatePEM imports a certificate from PEM format
func (cm *CertificateManager) ImportCertificatePEM(pemData string) (*x509.Certificate, error) {
	block, _ := pem.Decode([]byte(pemData))
	if block == nil {
		return nil, fmt.Errorf("failed to decode PEM block")
	}

	cert, err := x509.ParseCertificate(block.Bytes)
	if err != nil {
		return nil, fmt.Errorf("failed to parse certificate: %w", err)
	}

	return cert, nil
}

// GetCertificateInfo returns detailed information about a certificate
func (cm *CertificateManager) GetCertificateInfo(cert *x509.Certificate) *CertificateInfo {
	return &CertificateInfo{
		Subject:            cert.Subject.String(),
		Issuer:             cert.Issuer.String(),
		SerialNumber:       cert.SerialNumber.String(),
		NotBefore:          cert.NotBefore,
		NotAfter:           cert.NotAfter,
		DNSNames:           cert.DNSNames,
		IPAddresses:        cert.IPAddresses,
		KeyUsage:           cert.KeyUsage,
		ExtKeyUsage:        cert.ExtKeyUsage,
		IsCA:               cert.IsCA,
		Fingerprint:        cm.calculateFingerprint(cert),
		SignatureAlgorithm: cert.SignatureAlgorithm.String(),
		PublicKeyAlgorithm: cert.PublicKeyAlgorithm.String(),
	}
}

// CertificateInfo holds detailed certificate information
type CertificateInfo struct {
	Subject            string             `json:"subject"`
	Issuer             string             `json:"issuer"`
	SerialNumber       string             `json:"serial_number"`
	NotBefore          time.Time          `json:"not_before"`
	NotAfter           time.Time          `json:"not_after"`
	DNSNames           []string           `json:"dns_names"`
	IPAddresses        []net.IP           `json:"ip_addresses"`
	KeyUsage           x509.KeyUsage      `json:"key_usage"`
	ExtKeyUsage        []x509.ExtKeyUsage `json:"ext_key_usage"`
	IsCA               bool               `json:"is_ca"`
	Fingerprint        string             `json:"fingerprint"`
	SignatureAlgorithm string             `json:"signature_algorithm"`
	PublicKeyAlgorithm string             `json:"public_key_algorithm"`
}

// CleanupExpiredCache removes expired entries from the certificate cache
func (cm *CertificateManager) CleanupExpiredCache() int {
	cm.mutex.Lock()
	defer cm.mutex.Unlock()

	expired := 0
	expirationThreshold := time.Now().Add(-time.Duration(cm.config.CacheExpirationHours) * time.Hour)

	for key, cached := range cm.certificateCache {
		if cached.CachedAt.Before(expirationThreshold) {
			delete(cm.certificateCache, key)
			expired++
		}
	}

	return expired
}
