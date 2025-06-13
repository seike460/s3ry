package enterprise

import (
	"crypto/rand"
	"crypto/sha256"
	"encoding/base32"
	"encoding/hex"
	"fmt"
	"time"
)

// MFAProvider defines the interface for multi-factor authentication
type MFAProvider interface {
	GenerateSecret() (*MFASecret, error)
	ValidateToken(secret, token string) bool
	GenerateQRCodeURL(secret *MFASecret, userID, issuer string) string
	GenerateBackupCodes() ([]string, error)
	ValidateBackupCode(userID, code string) bool
}

// MFASecret represents a TOTP secret
type MFASecret struct {
	Secret    string    `json:"secret"`
	QRCode    string    `json:"qr_code,omitempty"`
	CreatedAt time.Time `json:"created_at"`
	Used      bool      `json:"used"`
}

// TOTPProvider implements TOTP-based MFA
type TOTPProvider struct {
	issuer string
}

// NewTOTPProvider creates a new TOTP MFA provider
func NewTOTPProvider(issuer string) *TOTPProvider {
	return &TOTPProvider{
		issuer: issuer,
	}
}

// GenerateSecret generates a new TOTP secret
func (t *TOTPProvider) GenerateSecret() (*MFASecret, error) {
	// Generate 20 random bytes for the secret
	secretBytes := make([]byte, 20)
	if _, err := rand.Read(secretBytes); err != nil {
		return nil, fmt.Errorf("failed to generate random secret: %w", err)
	}

	secret := base32.StdEncoding.EncodeToString(secretBytes)
	
	return &MFASecret{
		Secret:    secret,
		CreatedAt: time.Now(),
		Used:      false,
	}, nil
}

// ValidateToken validates a TOTP token
func (t *TOTPProvider) ValidateToken(secret, token string) bool {
	// Decode the secret
	decodedSecret, err := base32.StdEncoding.DecodeString(secret)
	if err != nil {
		return false
	}

	// Get current time step (30-second intervals)
	timeStep := time.Now().Unix() / 30

	// Check current time step and 1 step before/after for clock skew tolerance
	for i := -1; i <= 1; i++ {
		calculatedToken := generateTOTP(decodedSecret, timeStep+int64(i))
		if calculatedToken == token {
			return true
		}
	}

	return false
}

// GenerateQRCodeURL generates a QR code URL for TOTP setup
func (t *TOTPProvider) GenerateQRCodeURL(secret *MFASecret, userID, issuer string) string {
	return fmt.Sprintf("otpauth://totp/%s:%s?secret=%s&issuer=%s&algorithm=SHA1&digits=6&period=30",
		issuer, userID, secret.Secret, issuer)
}

// GenerateBackupCodes generates backup codes for MFA
func (t *TOTPProvider) GenerateBackupCodes() ([]string, error) {
	codes := make([]string, 10)
	
	for i := 0; i < 10; i++ {
		codeBytes := make([]byte, 8)
		if _, err := rand.Read(codeBytes); err != nil {
			return nil, fmt.Errorf("failed to generate backup code: %w", err)
		}
		
		// Format as 8-character hex string
		codes[i] = hex.EncodeToString(codeBytes)[:8]
	}
	
	return codes, nil
}

// ValidateBackupCode validates a backup code (implementation depends on storage)
func (t *TOTPProvider) ValidateBackupCode(userID, code string) bool {
	// This would need to be implemented with actual storage
	// For now, return false as backup codes need to be stored and managed
	return false
}

// generateTOTP generates a TOTP token for a given secret and time step
func generateTOTP(secret []byte, timeStep int64) string {
	// Convert time step to bytes
	timeBytes := make([]byte, 8)
	for i := 7; i >= 0; i-- {
		timeBytes[i] = byte(timeStep & 0xff)
		timeStep >>= 8
	}

	// HMAC-SHA1
	h := sha256.New()
	h.Write(secret)
	h.Write(timeBytes)
	hash := h.Sum(nil)

	// Dynamic truncation
	offset := hash[len(hash)-1] & 0x0f
	code := ((int(hash[offset]) & 0x7f) << 24) |
		((int(hash[offset+1]) & 0xff) << 16) |
		((int(hash[offset+2]) & 0xff) << 8) |
		(int(hash[offset+3]) & 0xff)

	// Generate 6-digit code
	code = code % 1000000
	return fmt.Sprintf("%06d", code)
}

// MFAConfig holds MFA configuration
type MFAConfig struct {
	Required      bool          `json:"required"`
	Provider      string        `json:"provider"`
	Issuer        string        `json:"issuer"`
	TokenWindow   time.Duration `json:"token_window"`
	BackupCodes   int           `json:"backup_codes"`
	GracePeriod   time.Duration `json:"grace_period"`
}

// DefaultMFAConfig returns default MFA configuration
func DefaultMFAConfig() *MFAConfig {
	return &MFAConfig{
		Required:      false,
		Provider:      "totp",
		Issuer:        "s3ry",
		TokenWindow:   time.Minute * 5,
		BackupCodes:   10,
		GracePeriod:   time.Hour * 24,
	}
}

// MFAManager manages MFA for users
type MFAManager struct {
	config   *MFAConfig
	provider MFAProvider
}

// NewMFAManager creates a new MFA manager
func NewMFAManager(config *MFAConfig) *MFAManager {
	if config == nil {
		config = DefaultMFAConfig()
	}

	var provider MFAProvider
	switch config.Provider {
	case "totp":
		provider = NewTOTPProvider(config.Issuer)
	default:
		provider = NewTOTPProvider(config.Issuer)
	}

	return &MFAManager{
		config:   config,
		provider: provider,
	}
}

// SetupMFA sets up MFA for a user
func (m *MFAManager) SetupMFA(userID string) (*MFASetupResponse, error) {
	secret, err := m.provider.GenerateSecret()
	if err != nil {
		return nil, fmt.Errorf("failed to generate MFA secret: %w", err)
	}

	qrCodeURL := m.provider.GenerateQRCodeURL(secret, userID, m.config.Issuer)
	
	backupCodes, err := m.provider.GenerateBackupCodes()
	if err != nil {
		return nil, fmt.Errorf("failed to generate backup codes: %w", err)
	}

	return &MFASetupResponse{
		Secret:      secret,
		QRCodeURL:   qrCodeURL,
		BackupCodes: backupCodes,
	}, nil
}

// ValidateMFA validates an MFA token
func (m *MFAManager) ValidateMFA(userID, secret, token string) bool {
	return m.provider.ValidateToken(secret, token)
}

// MFASetupResponse contains MFA setup information
type MFASetupResponse struct {
	Secret      *MFASecret `json:"secret"`
	QRCodeURL   string     `json:"qr_code_url"`
	BackupCodes []string   `json:"backup_codes"`
}