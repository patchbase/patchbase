package utils

import (
	"crypto/aes"
	"crypto/cipher"
	"crypto/rand"
	"crypto/sha256"
	"encoding/base64"
	"encoding/hex"
	"fmt"
	"io"

	"github.com/samber/do/v2"
	"go.patchbase.net/server/internal/config"
)

// SHA256 returns the hexadecimal representation of the SHA-256 hash of the input string.
func SHA256(input string) string {
	sum := sha256.Sum256([]byte(input))
	return hex.EncodeToString(sum[:])
}

type Crypto interface {
	Encrypt(plainText string) (string, error)
	Decrypt(cipherTextBase64 string) (string, error)
}

type crypto struct {
	gcm cipher.AEAD
}

func NewCrypto(i do.Injector) (Crypto, error) {
	cfg, err := do.Invoke[config.Config](i)
	if err != nil {
		return nil, fmt.Errorf("failed to get config: %w", err)
	}
	return NewCryptoWithKey(cfg.EncryptionKey)
}

func NewCryptoWithKey(secretKey string) (Crypto, error) {
	if secretKey == "" {
		return nil, fmt.Errorf("encryption secret key cannot be empty")
	}

	// Derive 32-byte key using SHA-256
	hasher := sha256.New()
	hasher.Write([]byte(secretKey))
	key := hasher.Sum(nil)
	block, err := aes.NewCipher(key)
	if err != nil {
		return nil, fmt.Errorf("failed to create aes cipher: %w", err)
	}

	gcm, err := cipher.NewGCM(block)
	if err != nil {
		return nil, fmt.Errorf("failed to create gcm: %w", err)
	}

	return &crypto{gcm: gcm}, nil
}

// Encrypt encrypts plainText using AES-GCM with a key derived from secretKey.
// The output is a base64-encoded string containing the nonce and ciphertext.
func (c *crypto) Encrypt(plainText string) (string, error) {
	nonce := make([]byte, c.gcm.NonceSize())
	if _, err := io.ReadFull(rand.Reader, nonce); err != nil {
		return "", fmt.Errorf("generate nonce: %w", err)
	}

	sealed := c.gcm.Seal(nil, nonce, []byte(plainText), nil)

	// Combine nonce + ciphertext
	combined := append(nonce, sealed...)

	return base64.StdEncoding.EncodeToString(combined), nil
}

// Decrypt decrypts a base64-encoded ciphertext using AES-GCM with a key derived from secretKey.
func (c *crypto) Decrypt(cipherTextBase64 string) (string, error) {
	combined, err := base64.StdEncoding.DecodeString(cipherTextBase64)
	if err != nil {
		return "", fmt.Errorf("decode base64: %w", err)
	}

	nonceSize := c.gcm.NonceSize()
	if len(combined) < nonceSize {
		return "", fmt.Errorf("ciphertext too short")
	}

	nonce := combined[:nonceSize]
	sealed := combined[nonceSize:]

	decrypted, err := c.gcm.Open(nil, nonce, sealed, nil)
	if err != nil {
		return "", fmt.Errorf("decrypt failed: %w", err)
	}

	return string(decrypted), nil
}
