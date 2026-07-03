#!/usr/bin/env bash
set -euo pipefail

cd "$(dirname "$0")/.."

GPG_KEY="${PATCHBASE_PACKAGE_SIGNING_KEY_FINGERPRINT:-}"
if [ -z "$GPG_KEY" ]; then
    echo "Error: PATCHBASE_PACKAGE_SIGNING_KEY_FINGERPRINT is not set." >&2
    exit 1
fi

# Write rpm macros for non-interactive signing
cat <<EOF > ~/.rpmmacros
%_gpg_name $GPG_KEY
%__gpg_sign_cmd %{__gpg} gpg --batch --yes --pinentry-mode loopback --no-armor --no-secmem-warning -u "%{_gpg_name}" -sbo %{__signature_filename} %{__plaintext_filename}
EOF

echo "Setting up dist/repo layout..."

EL_VERSIONS=(9 10)
ARCHES=(x86_64 aarch64)

shopt -s nullglob

for EL_VER in "${EL_VERSIONS[@]}"; do
    for ARCH in "${ARCHES[@]}"; do
        REPO_DIR="dist/repo/rpm/el/$EL_VER/$ARCH"
        mkdir -p "$REPO_DIR"

        echo "Copying RPMs to $REPO_DIR..."
        rpms=(dist/rpm/*"$ARCH"*.rpm)
        if [ ${#rpms[@]} -eq 0 ]; then
            echo "Error: no RPMs found for architecture $ARCH in dist/rpm/" >&2
            exit 1
        fi
        cp "${rpms[@]}" "$REPO_DIR"/

        echo "Signing copied RPMs in $REPO_DIR..."
        rpm --addsign "$REPO_DIR"/*.rpm

        echo "Generating createrepo metadata for $REPO_DIR..."
        createrepo_c "$REPO_DIR"

        echo "Signing repository metadata for $REPO_DIR..."
        gpg --detach-sign --armor --local-user "$GPG_KEY" --yes "$REPO_DIR/repodata/repomd.xml"
    done
done

echo "Copying patchbase.repo to dist/repo/..."
cp packaging/patchbase.repo dist/repo/

echo "Repository generation complete."

echo "Verifying signatures..."
for EL_VER in "${EL_VERSIONS[@]}"; do
    for ARCH in "${ARCHES[@]}"; do
        REPO_DIR="dist/repo/rpm/el/$EL_VER/$ARCH"
        rpm --checksig "$REPO_DIR"/*.rpm
        createrepo_c --checkts "$REPO_DIR"
        gpg --verify "$REPO_DIR/repodata/repomd.xml.asc" "$REPO_DIR/repodata/repomd.xml"
    done
done

echo "All verifications passed!"
