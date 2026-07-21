// SPDX-FileCopyrightText: 2026 Configure Labs SRL
// SPDX-License-Identifier: AGPL-3.0-only
package utils

import (
	"crypto/ed25519"
	crand "crypto/rand"
	"crypto/x509"
	"encoding/pem"
	"fmt"
	"strings"

	"golang.org/x/crypto/ssh"
)

func GenerateSSHKeyPair() (string, string, error) {
	publicKey, privateKey, err := ed25519.GenerateKey(crand.Reader)
	if err != nil {
		return "", "", fmt.Errorf("generate ed25519 key pair: %w", err)
	}

	sshPublicKey, err := ssh.NewPublicKey(publicKey)
	if err != nil {
		return "", "", fmt.Errorf("convert public key: %w", err)
	}
	publicAuthorizedKey := strings.TrimSpace(string(ssh.MarshalAuthorizedKey(sshPublicKey)))

	der, err := x509.MarshalPKCS8PrivateKey(privateKey)
	if err != nil {
		return "", "", fmt.Errorf("marshal private key: %w", err)
	}
	privatePEM := pem.EncodeToMemory(&pem.Block{Type: "PRIVATE KEY", Bytes: der}) // nolint: exhaustruct
	if privatePEM == nil {
		return "", "", fmt.Errorf("encode private key pem")
	}

	return publicAuthorizedKey, string(privatePEM), nil
}
