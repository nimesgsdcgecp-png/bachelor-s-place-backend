// Package crypto provides AES-256-GCM encryption and decryption for PII fields.
// Used for: phone_encrypted, aadhaar_encrypted, pan_encrypted.
//
// The key must be a 64-character hex string (32 bytes).
// Encrypted output is hex-encoded and includes the random nonce as a prefix,
// making every call to Encrypt produce a different ciphertext for the same input.
package crypto

import (
	"crypto/aes"
	"crypto/cipher"
	"crypto/rand"
	"encoding/hex"
	"fmt"
	"io"
)

// Encryptor holds a prepared AES-256-GCM cipher.
// Create one at startup and reuse across all requests.
type Encryptor struct {
	gcm cipher.AEAD
}

// New creates an Encryptor from a 64-character hex-encoded 32-byte key.
// Typically called once in main.go using cfg.EncryptionKey.
func New(hexKey string) (*Encryptor, error) {
	keyBytes, err := hex.DecodeString(hexKey)
	if err != nil {
		return nil, fmt.Errorf("crypto: invalid key (not valid hex): %w", err)
	}
	if len(keyBytes) != 32 {
		return nil, fmt.Errorf("crypto: key must be 32 bytes, got %d", len(keyBytes))
	}

	block, err := aes.NewCipher(keyBytes)
	if err != nil {
		return nil, fmt.Errorf("crypto: failed to create AES cipher: %w", err)
	}

	gcm, err := cipher.NewGCM(block)
	if err != nil {
		return nil, fmt.Errorf("crypto: failed to create GCM: %w", err)
	}

	return &Encryptor{gcm: gcm}, nil
}

// Encrypt encrypts plaintext using AES-256-GCM.
// Returns a hex-encoded string containing the nonce + ciphertext.
// The same plaintext will produce different output on each call (random nonce).
func (e *Encryptor) Encrypt(plaintext string) (string, error) {
	nonce := make([]byte, e.gcm.NonceSize())
	if _, err := io.ReadFull(rand.Reader, nonce); err != nil {
		return "", fmt.Errorf("crypto: failed to generate nonce: %w", err)
	}
	// Seal appends ciphertext to nonce so both are stored together
	sealed := e.gcm.Seal(nonce, nonce, []byte(plaintext), nil)
	return hex.EncodeToString(sealed), nil
}

// Decrypt decrypts a hex-encoded ciphertext produced by Encrypt.
func (e *Encryptor) Decrypt(hexCiphertext string) (string, error) {
	data, err := hex.DecodeString(hexCiphertext)
	if err != nil {
		return "", fmt.Errorf("crypto: invalid ciphertext (not valid hex): %w", err)
	}

	nonceSize := e.gcm.NonceSize()
	if len(data) < nonceSize {
		return "", fmt.Errorf("crypto: ciphertext too short")
	}

	nonce, ciphertext := data[:nonceSize], data[nonceSize:]
	plaintext, err := e.gcm.Open(nil, nonce, ciphertext, nil)
	if err != nil {
		return "", fmt.Errorf("crypto: decryption failed (wrong key or corrupt data): %w", err)
	}

	return string(plaintext), nil
}
