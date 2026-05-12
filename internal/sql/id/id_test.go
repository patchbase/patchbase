package id

import (
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestGeneratesID(t *testing.T) {
	id := New("u")
	assert.True(t, strings.HasPrefix(id, "u_"))
	assert.Len(t, id, 16)
}
