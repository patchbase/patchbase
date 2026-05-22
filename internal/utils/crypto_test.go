package utils_test

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.patchbase.net/server/internal/utils"
)

func TestEncryptDecrypt(t *testing.T) {
	plainText := "my-secret-payload"
	crypto, err := utils.NewCryptoWithKey("my-secret-key")
	require.NoError(t, err, "failed to create crypto instance")

	cipherText, err := crypto.Encrypt(plainText)
	require.NoError(t, err, "encryption should succeed")
	assert.NotEmpty(t, cipherText, "ciphertext should not be empty")
	assert.NotEqual(t, plainText, cipherText, "ciphertext should differ from plaintext")

	decrypted, err := crypto.Decrypt(cipherText)
	require.NoError(t, err, "decryption should succeed")
	assert.Equal(t, plainText, decrypted, "decrypted text should match original plaintext")
}

func TestDecryptInvalidKey(t *testing.T) {
	secretKey1 := "key-1"
	secretKey2 := "key-2"
	plainText := "my-secret-payload"
	crypto1, err := utils.NewCryptoWithKey(secretKey1)
	require.NoError(t, err, "failed to create crypto instance with key 1")
	crypto2, err := utils.NewCryptoWithKey(secretKey2)
	require.NoError(t, err, "failed to create crypto instance with key 2")

	cipherText, err := crypto1.Encrypt(plainText)
	require.NoError(t, err, "encryption should succeed")

	_, err = crypto2.Decrypt(cipherText)
	assert.Error(t, err, "decrypting with wrong key should fail")
}
