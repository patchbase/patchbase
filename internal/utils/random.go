// SPDX-FileCopyrightText: 2026 Configure Labs SRL
// SPDX-License-Identifier: AGPL-3.0-only
package utils

import (
	"crypto/rand"
	"encoding/hex"
	"fmt"
	mathrand "math/rand"
)

var (
	alphabet = []rune("abcdefghijklmnopqrstuvwxyzABCDEFGHIJKLMNOPQRSTUVWXYZ0123456789")
)

type RandomStringGenerator interface {
	// New generates a random string of the specified length using the defined alphabet.
	New(length int) string
	// Hex generates a random hexadecimal string of the specified length.
	Hex(length int) string
}

type randomStringGenerator struct{}

func NewRandomStringGenerator() RandomStringGenerator {
	return randomStringGenerator{}
}

// New generates a random string of the specified length using the defined alphabet.
// It uses math/rand for random number generation, which is not suitable for cryptographic purposes.
// For secure random strings, consider using the Hex method instead.
func (randomStringGenerator) New(length int) string {
	b := make([]rune, length)
	for i := range b {
		b[i] = alphabet[mathrand.Intn(len(alphabet))] // nolint: gosec
	}
	return string(b)
}

func (randomStringGenerator) Hex(length int) string {
	return RandomHex(length)
}

func RandomHex(length int) string {
	buf := make([]byte, length)
	if _, err := rand.Read(buf); err != nil {
		panic(fmt.Errorf("read random bytes: %w", err))
	}

	return hex.EncodeToString(buf)
}
