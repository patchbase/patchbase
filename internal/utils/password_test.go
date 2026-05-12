package utils

import (
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestHashPassword(t *testing.T) {
	password := "test-password"

	hash, err := HashPassword(password)
	require.NoError(t, err)

	assert.NotEmpty(t, hash)
	assert.True(t, strings.HasPrefix(hash, "$2"))
	assert.True(t, CheckPasswordHash(password, hash))
	assert.False(t, CheckPasswordHash("wrong-password", hash))
}

func TestCheckPasswordHashRejectsMalformedHash(t *testing.T) {
	assert.False(t, CheckPasswordHash("test-password", "invalid"))
}
