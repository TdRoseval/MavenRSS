package crypto

import (
	"crypto/aes"
	"crypto/cipher"
	"crypto/rand"
	"crypto/sha256"
	"encoding/base64"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"runtime"
	"strings"

	"golang.org/x/crypto/pbkdf2"
)

const (
	// Number of iterations for PBKDF2 (OWASP recommends 600,000+ for PBKDF2-HMAC-SHA256 as of 2023)
	pbkdf2Iterations = 600000
	// Key size for AES-256
	keySize = 32
	// Salt size for key derivation
	saltSize = 16
	// Version marker to identify encrypted values (prevents false positives in IsEncrypted)
	versionMarker = "MrRSS-v1:"
)

var (
	// ErrInvalidCiphertext is returned when the ciphertext is too short or invalid
	ErrInvalidCiphertext = errors.New("invalid ciphertext")
	// ErrDecryptionFailed is returned when decryption fails
	ErrDecryptionFailed = errors.New("decryption failed")
	// serverModeKeyDir is the directory where the server mode key file is stored
	serverModeKeyDir string
	// serverModeKeyFileName is the name of the server mode key file
	serverModeKeyFileName = "encryption.key"
)

// SetServerModeKeyDir sets the directory where the server mode key file is stored
func SetServerModeKeyDir(dir string) {
	serverModeKeyDir = dir
}

// getServerModeKey reads or generates the server mode encryption key
func getServerModeKey() ([]byte, error) {
	if serverModeKeyDir == "" {
		return nil, fmt.Errorf("server mode key directory not set")
	}
	
	keyPath := filepath.Join(serverModeKeyDir, serverModeKeyFileName)
	
	// Check if key file exists
	if _, err := os.Stat(keyPath); err == nil {
		// Read existing key
		key, err := os.ReadFile(keyPath)
		if err != nil {
			return nil, fmt.Errorf("failed to read encryption key: %w", err)
		}
		if len(key) == keySize {
			return key, nil
		}
		// If key is wrong size, generate new one
	}
	
	// Generate new key
	key := make([]byte, keySize)
	if _, err := io.ReadFull(rand.Reader, key); err != nil {
		return nil, fmt.Errorf("failed to generate encryption key: %w", err)
	}
	
	// Ensure directory exists
	if err := os.MkdirAll(serverModeKeyDir, 0700); err != nil {
		return nil, fmt.Errorf("failed to create key directory: %w", err)
	}
	
	// Write key to file
	if err := os.WriteFile(keyPath, key, 0600); err != nil {
		return nil, fmt.Errorf("failed to write encryption key: %w", err)
	}
	
	return key, nil
}

// GetMachineID generates a machine-specific identifier for key derivation.
// This ensures that encrypted data is tied to the specific machine.
// It combines multiple sources of entropy for better security.
func GetMachineID() (string, error) {
	// If in server mode with key directory set, we won't use this
	if serverModeKeyDir != "" {
		// For backward compatibility, still return something
		return "server-mode", nil
	}
	
	// Use hostname as the primary identifier
	hostname, err := os.Hostname()
	if err != nil {
		return "", fmt.Errorf("failed to get hostname: %w", err)
	}

	// Try to get machine-id from various system locations (Linux/BSD)
	machineUUID := ""
	possiblePaths := []string{
		"/etc/machine-id",
		"/var/lib/dbus/machine-id",
	}
	for _, path := range possiblePaths {
		if data, err := os.ReadFile(path); err == nil {
			machineUUID = string(data)
			break
		}
	}

	// Combine multiple sources: hostname, OS, architecture, and machine UUID
	// This provides better entropy than hostname alone
	machineID := fmt.Sprintf("%s-%s-%s-%s", hostname, runtime.GOOS, runtime.GOARCH, machineUUID)

	return machineID, nil
}

// DeriveKey derives a cryptographic key.
// In server mode, uses the fixed key from file.
// Otherwise, derives from machine ID using PBKDF2.
func DeriveKey(machineID string, salt []byte) []byte {
	if serverModeKeyDir != "" {
		key, err := getServerModeKey()
		if err == nil {
			return key
		}
		// Fall back to PBKDF2 if key file fails
	}
	return pbkdf2.Key([]byte(machineID), salt, pbkdf2Iterations, keySize, sha256.New)
}

// Encrypt encrypts plaintext using AES-256-GCM with a machine-specific key.
// The output format is: [salt(16 bytes)][nonce(12 bytes)][ciphertext+tag]
// Returns base64-encoded result for safe storage in database.
func Encrypt(plaintext string) (string, error) {
	if plaintext == "" {
		return "", nil
	}

	// Get machine ID for key derivation
	machineID, err := GetMachineID()
	if err != nil {
		return "", fmt.Errorf("failed to get machine ID: %w", err)
	}

	// Generate random salt
	salt := make([]byte, saltSize)
	if _, err := io.ReadFull(rand.Reader, salt); err != nil {
		return "", fmt.Errorf("failed to generate salt: %w", err)
	}

	// Derive encryption key
	key := DeriveKey(machineID, salt)

	// Create AES cipher
	block, err := aes.NewCipher(key)
	if err != nil {
		return "", fmt.Errorf("failed to create cipher: %w", err)
	}

	// Create GCM mode
	gcm, err := cipher.NewGCM(block)
	if err != nil {
		return "", fmt.Errorf("failed to create GCM: %w", err)
	}

	// Generate random nonce
	nonce := make([]byte, gcm.NonceSize())
	if _, err := io.ReadFull(rand.Reader, nonce); err != nil {
		return "", fmt.Errorf("failed to generate nonce: %w", err)
	}

	// Encrypt the plaintext
	ciphertext := gcm.Seal(nil, nonce, []byte(plaintext), nil)

	// Combine salt, nonce, and ciphertext
	result := make([]byte, 0, saltSize+len(nonce)+len(ciphertext))
	result = append(result, salt...)
	result = append(result, nonce...)
	result = append(result, ciphertext...)

	// Encode to base64 and prepend version marker for safe storage
	encoded := base64.StdEncoding.EncodeToString(result)
	return versionMarker + encoded, nil
}

// Decrypt decrypts ciphertext that was encrypted with Encrypt.
// The input must be version-prefixed base64-encoded and contain: [salt][nonce][ciphertext+tag]
func Decrypt(ciphertextBase64 string) (string, error) {
	if ciphertextBase64 == "" {
		return "", nil
	}

	// Check and strip version marker
	if !strings.HasPrefix(ciphertextBase64, versionMarker) {
		return "", fmt.Errorf("missing or invalid version marker")
	}
	ciphertextBase64 = strings.TrimPrefix(ciphertextBase64, versionMarker)

	// Decode from base64
	data, err := base64.StdEncoding.DecodeString(ciphertextBase64)
	if err != nil {
		return "", fmt.Errorf("failed to decode base64: %w", err)
	}

	// Check minimum length: salt + nonce + tag (at least)
	if len(data) < saltSize+12+16 {
		return "", ErrInvalidCiphertext
	}

	// Extract salt
	salt := data[:saltSize]

	// Get machine ID for key derivation
	machineID, err := GetMachineID()
	if err != nil {
		return "", fmt.Errorf("failed to get machine ID: %w", err)
	}

	// Derive decryption key
	key := DeriveKey(machineID, salt)

	// Create AES cipher
	block, err := aes.NewCipher(key)
	if err != nil {
		return "", fmt.Errorf("failed to create cipher: %w", err)
	}

	// Create GCM mode
	gcm, err := cipher.NewGCM(block)
	if err != nil {
		return "", fmt.Errorf("failed to create GCM: %w", err)
	}

	// Extract nonce and ciphertext
	nonceSize := gcm.NonceSize()
	if len(data) < saltSize+nonceSize {
		return "", ErrInvalidCiphertext
	}

	nonce := data[saltSize : saltSize+nonceSize]
	ciphertext := data[saltSize+nonceSize:]

	// Decrypt the ciphertext
	plaintext, err := gcm.Open(nil, nonce, ciphertext, nil)
	if err != nil {
		return "", ErrDecryptionFailed
	}

	return string(plaintext), nil
}

// IsEncrypted checks if a value appears to be encrypted by checking for the version marker.
// This definitively identifies encrypted values and prevents false positives.
func IsEncrypted(value string) bool {
	if value == "" {
		return false
	}

	// Check for version marker - this is definitive, not a heuristic
	return strings.HasPrefix(value, versionMarker)
}
