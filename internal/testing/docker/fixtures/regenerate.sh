#!/usr/bin/env bash
# Regenerate the test SSH keypair fixtures used by the docker-based integration
# tests in internal/testing/e2e. After running this script, the new keypair is
# committed at internal/testing/docker/fixtures/.
#
# Note: The authorized_keys file is NOT regenerated here. The test harness
# injects the server-generated public key at runtime via WriteAuthorizedKey,
# so there is no baked-in authorized_keys to keep in sync.
set -euo pipefail

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
TMP_DIR="$(mktemp -d)"
trap 'rm -rf "${TMP_DIR}"' EXIT

ssh-keygen -t ed25519 -f "${TMP_DIR}/id_ed25519" -N "" -C "patchbase-test"

cp "${TMP_DIR}/id_ed25519"     "${SCRIPT_DIR}/id_ed25519"
cp "${TMP_DIR}/id_ed25519.pub" "${SCRIPT_DIR}/id_ed25519.pub"

chmod 600 "${SCRIPT_DIR}/id_ed25519"

echo "Regenerated ${SCRIPT_DIR}/id_ed25519{,.pub}"
