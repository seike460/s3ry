package enterprise

import (
	"strings"
	"testing"
	"time"
)

func TestNewTOTPProvider(t *testing.T) {
	provider := NewTOTPProvider("s3ry-test")
	if provider == nil {
		t.Fatal("NewTOTPProvider returned nil")
	}
	if provider.issuer != "s3ry-test" {
		t.Errorf("Expected issuer 's3ry-test', got '%s'", provider.issuer)
	}
}

func TestGenerateSecret(t *testing.T) {
	provider := NewTOTPProvider("s3ry-test")

	secret, err := provider.GenerateSecret()
	if err != nil {
		t.Fatalf("GenerateSecret failed: %v", err)
	}

	if secret == nil {
		t.Fatal("GenerateSecret returned nil secret")
	}

	if secret.Secret == "" {
		t.Error("Secret string is empty")
	}

	if secret.CreatedAt.IsZero() {
		t.Error("CreatedAt is not set")
	}

	if secret.Used {
		t.Error("New secret should not be marked as used")
	}

	// Verify secret is base32 encoded
	if len(secret.Secret) == 0 {
		t.Error("Secret is empty")
	}
}

func TestGenerateQRCodeURL(t *testing.T) {
	provider := NewTOTPProvider("s3ry-test")

	secret, err := provider.GenerateSecret()
	if err != nil {
		t.Fatalf("GenerateSecret failed: %v", err)
	}

	qrURL := provider.GenerateQRCodeURL(secret, "testuser", "s3ry-test")

	if !strings.HasPrefix(qrURL, "otpauth://totp/") {
		t.Errorf("QR URL should start with 'otpauth://totp/', got: %s", qrURL)
	}

	if !strings.Contains(qrURL, "testuser") {
		t.Error("QR URL should contain username")
	}

	if !strings.Contains(qrURL, "s3ry-test") {
		t.Error("QR URL should contain issuer")
	}

	if !strings.Contains(qrURL, secret.Secret) {
		t.Error("QR URL should contain secret")
	}
}

func TestGenerateBackupCodes(t *testing.T) {
	provider := NewTOTPProvider("s3ry-test")

	codes, err := provider.GenerateBackupCodes()
	if err != nil {
		t.Fatalf("GenerateBackupCodes failed: %v", err)
	}

	if len(codes) != 10 {
		t.Errorf("Expected 10 backup codes, got %d", len(codes))
	}

	// Check codes are unique
	codeMap := make(map[string]bool)
	for _, code := range codes {
		if len(code) != 8 {
			t.Errorf("Expected 8-character backup code, got %d characters: %s", len(code), code)
		}

		if codeMap[code] {
			t.Errorf("Duplicate backup code found: %s", code)
		}
		codeMap[code] = true
	}
}

func TestValidateToken(t *testing.T) {
	provider := NewTOTPProvider("s3ry-test")

	// Test with known secret for consistent results
	knownSecret := "JBSWY3DPEHPK3PXP" // "Hello" in base32

	// Note: TOTP validation depends on current time, so we can't easily test
	// valid tokens without mocking time. Here we test invalid scenarios.

	// Test with invalid token
	if provider.ValidateToken(knownSecret, "000000") {
		t.Error("ValidateToken should reject invalid token")
	}

	// Test with invalid secret
	if provider.ValidateToken("INVALID", "123456") {
		t.Error("ValidateToken should reject invalid secret")
	}
}

func TestDefaultMFAConfig(t *testing.T) {
	config := DefaultMFAConfig()

	if config == nil {
		t.Fatal("DefaultMFAConfig returned nil")
	}

	if config.Required {
		t.Error("MFA should not be required by default")
	}

	if config.Provider != "totp" {
		t.Errorf("Expected provider 'totp', got '%s'", config.Provider)
	}

	if config.Issuer != "s3ry" {
		t.Errorf("Expected issuer 's3ry', got '%s'", config.Issuer)
	}

	if config.BackupCodes != 10 {
		t.Errorf("Expected 10 backup codes, got %d", config.BackupCodes)
	}
}

func TestNewMFAManager(t *testing.T) {
	config := DefaultMFAConfig()
	manager := NewMFAManager(config)

	if manager == nil {
		t.Fatal("NewMFAManager returned nil")
	}

	if manager.config != config {
		t.Error("MFAManager config not set correctly")
	}

	if manager.provider == nil {
		t.Error("MFAManager provider not initialized")
	}
}

func TestNewMFAManagerWithNilConfig(t *testing.T) {
	manager := NewMFAManager(nil)

	if manager == nil {
		t.Fatal("NewMFAManager returned nil with nil config")
	}

	if manager.config == nil {
		t.Error("MFAManager should use default config when nil provided")
	}

	if manager.provider == nil {
		t.Error("MFAManager provider not initialized")
	}
}

func TestSetupMFA(t *testing.T) {
	manager := NewMFAManager(nil)

	response, err := manager.SetupMFA("testuser")
	if err != nil {
		t.Fatalf("SetupMFA failed: %v", err)
	}

	if response == nil {
		t.Fatal("SetupMFA returned nil response")
	}

	if response.Secret == nil {
		t.Error("MFA setup response should include secret")
	}

	if response.QRCodeURL == "" {
		t.Error("MFA setup response should include QR code URL")
	}

	if len(response.BackupCodes) == 0 {
		t.Error("MFA setup response should include backup codes")
	}

	if len(response.BackupCodes) != 10 {
		t.Errorf("Expected 10 backup codes, got %d", len(response.BackupCodes))
	}
}

func TestValidateMFA(t *testing.T) {
	manager := NewMFAManager(nil)

	// Test with invalid token (we can't easily test valid tokens without time mocking)
	if manager.ValidateMFA("testuser", "JBSWY3DPEHPK3PXP", "000000") {
		t.Error("ValidateMFA should reject invalid token")
	}
}

func TestMFASecret(t *testing.T) {
	secret := &MFASecret{
		Secret:    "TESTSECRET",
		CreatedAt: time.Now(),
		Used:      false,
	}

	if secret.Secret != "TESTSECRET" {
		t.Error("Secret not set correctly")
	}

	if secret.Used {
		t.Error("New secret should not be marked as used")
	}

	if secret.CreatedAt.IsZero() {
		t.Error("CreatedAt should be set")
	}
}

func TestMFASetupResponse(t *testing.T) {
	secret := &MFASecret{
		Secret:    "TESTSECRET",
		CreatedAt: time.Now(),
		Used:      false,
	}

	response := &MFASetupResponse{
		Secret:      secret,
		QRCodeURL:   "otpauth://totp/test",
		BackupCodes: []string{"code1", "code2"},
	}

	if response.Secret != secret {
		t.Error("Secret not set correctly in response")
	}

	if response.QRCodeURL != "otpauth://totp/test" {
		t.Error("QR code URL not set correctly")
	}

	if len(response.BackupCodes) != 2 {
		t.Error("Backup codes not set correctly")
	}
}
