// SPDX-FileCopyrightText: 2026 Configure Labs SRL
// SPDX-License-Identifier: AGPL-3.0-only
package utils

import (
	"fmt"

	"golang.org/x/crypto/bcrypt"
)

const (
	// bcryptCost stays above the library default without making auth/setup requests stall.
	bcryptCost = 12
)

// HashPassword returns a bcrypt hash suitable for storage.
func HashPassword(password string) (string, error) {
	hash, err := bcrypt.GenerateFromPassword([]byte(password), bcryptCost)
	if err != nil {
		return "", fmt.Errorf("generate bcrypt password hash: %w", err)
	}

	return string(hash), nil
}

// CheckPasswordHash verifies a password against a stored hash.
func CheckPasswordHash(password string, hash string) bool {
	return bcrypt.CompareHashAndPassword([]byte(hash), []byte(password)) == nil
}
