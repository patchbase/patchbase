package client

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestSnapshotEndpoint(t *testing.T) {
	result := snapshotEndpoint("https://patchbase.local/")
	assert.Equal(t, "https://patchbase.local/api/v1/agent/snapshots", result)

	result = snapshotEndpoint("https://patchbase.local")
	assert.Equal(t, "https://patchbase.local/api/v1/agent/snapshots", result)

	result = snapshotEndpoint("https://patchbase.local/api/v1/agent/snapshots")
	assert.Equal(t, "https://patchbase.local/api/v1/agent/snapshots", result)
}

func TestIsLoopback(t *testing.T) {
	assert.True(t, isLoopback("http://localhost/api"))
	assert.True(t, isLoopback("http://127.0.0.1/api"))
	assert.True(t, isLoopback("http://[::1]/api"))
	assert.False(t, isLoopback("https://patchbase.local/api"))
}
