#!/usr/bin/env bash
set -euo pipefail

cd "$(dirname "$0")/.."

GPG_KEY="${PATCHBASE_PACKAGE_SIGNING_KEY_FINGERPRINT:-}"
if [ -z "$GPG_KEY" ]; then
    echo "Error: PATCHBASE_PACKAGE_SIGNING_KEY_FINGERPRINT is not set." >&2
    exit 1
fi

PASSPHRASE="${PATCHBASE_PACKAGE_SIGNING_KEY_PASSPHRASE:-}"
if [ -z "$PASSPHRASE" ]; then
    echo "Error: PATCHBASE_PACKAGE_SIGNING_KEY_PASSPHRASE is not set." >&2
    exit 1
fi

GNUPGHOME="${GNUPGHOME:-$HOME/.gnupg}"
mkdir -p "$GNUPGHOME"
chmod 700 "$GNUPGHOME"

# Allow loopback pinentry so passphrase-based signing works non-interactively.
gpg_conf="$GNUPGHOME/gpg.conf"
touch "$gpg_conf"
grep -q '^pinentry-mode loopback$' "$gpg_conf" || echo "pinentry-mode loopback" >> "$gpg_conf"

agent_conf="$GNUPGHOME/gpg-agent.conf"
touch "$agent_conf"
grep -q '^allow-loopback-pinentry$' "$agent_conf" || echo "allow-loopback-pinentry" >> "$agent_conf"
# Restart agent so it picks up the new config.
gpgconf --kill gpg-agent 2>/dev/null || true
gpgconf --launch gpg-agent 2>/dev/null || true

# Wrapper that injects the passphrase for every gpg invocation so rpm and
# reprepro (which call gpg internally) can sign without prompting.
GPG_WRAPPER="$GNUPGHOME/gpg-wrapper"
cat > "$GPG_WRAPPER" <<EOF
#!/usr/bin/env bash
exec /usr/bin/gpg \\
  --pinentry-mode loopback \\
  --passphrase-fd 3 \\
  "\$@" 3<<'__PB_PASSPHRASE__'
${PASSPHRASE}
__PB_PASSPHRASE__
EOF
chmod 700 "$GPG_WRAPPER"

# Write rpm macros for non-interactive signing.
cat <<EOF > ~/.rpmmacros
%_gpg_name ${GPG_KEY}
%__gpg ${GPG_WRAPPER}
%_gpg_sign_cmd_extra_args --batch --yes
EOF

# Wrapper used by this script for direct gpg calls.
sign_gpg() {
    "$GPG_WRAPPER" "$@"
}

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
        chmod u+w "$REPO_DIR"/*.rpm

        echo "Signing copied RPMs in $REPO_DIR..."
        rpm --addsign "$REPO_DIR"/*.rpm

        echo "Generating createrepo metadata for $REPO_DIR..."
        createrepo_c "$REPO_DIR"

        echo "Signing repository metadata for $REPO_DIR..."
        sign_gpg --detach-sign --armor --local-user "$GPG_KEY" --yes \
            --output "$REPO_DIR/repodata/repomd.xml.asc" \
            "$REPO_DIR/repodata/repomd.xml"
    done
done

echo "Copying patchbase.repo to dist/repo/..."
cp packaging/patchbase.repo dist/repo/

# Debian Repository Generation
echo "Setting up dist/repo/deb layout..."
DEB_REPO_DIR="dist/repo/deb"
mkdir -p "$DEB_REPO_DIR/conf"

cat <<EOF > "$DEB_REPO_DIR/conf/distributions"
Origin: Patchbase
Label: Patchbase
Codename: stable
Architectures: amd64 arm64
Components: main
Description: Patchbase APT Repository
SignWith: ${GPG_KEY}
EOF

# reprepro calls gpg internally; make sure it uses the wrapper too.
echo "Including DEB packages into reprepro repository..."
for DEB_ARCH in amd64 arm64; do
    arch_debs=(dist/deb/*_"$DEB_ARCH".deb)
    if [ ${#arch_debs[@]} -eq 0 ]; then
        echo "Error: no DEB packages found for architecture $DEB_ARCH in dist/deb/" >&2
        exit 1
    fi
    PATH="$GNUPGHOME:$PATH" reprepro --basedir "$DEB_REPO_DIR" includedeb stable "${arch_debs[@]}"
done

echo "Copying patchbase.list to dist/repo/..."
cp packaging/patchbase.list dist/repo/

echo "Repository generation complete."

# Set up an isolated rpmdb so verification works on distros without /var/lib/rpm
# and without requiring root. Import the public key into it so --checksig can
# actually validate signatures.
VERIFY_RPMDB="$(mktemp -d)"
trap 'rm -rf "$VERIFY_RPMDB"' EXIT
rpm --dbpath "$VERIFY_RPMDB" --import dist/repo/keys/patchbase.asc

echo "Verifying signatures..."
for EL_VER in "${EL_VERSIONS[@]}"; do
    for ARCH in "${ARCHES[@]}"; do
        REPO_DIR="dist/repo/rpm/el/$EL_VER/$ARCH"
        rpm --dbpath "$VERIFY_RPMDB" --checksig "$REPO_DIR"/*.rpm
        gpg --verify "$REPO_DIR/repodata/repomd.xml.asc" "$REPO_DIR/repodata/repomd.xml"
    done
done

echo "Verifying Debian metadata..."
TMP_LIST="$(mktemp)"
trap 'rm -rf "$VERIFY_RPMDB" "$TMP_LIST"' EXIT
reprepro --basedir "$DEB_REPO_DIR" list stable > "$TMP_LIST"
cat "$TMP_LIST"
grep -q "patchbase-server" "$TMP_LIST" || { echo "Error: patchbase-server missing from Debian repo"; exit 1; }
grep -q "patchbase-agent" "$TMP_LIST" || { echo "Error: patchbase-agent missing from Debian repo"; exit 1; }
gpg --verify "$DEB_REPO_DIR/dists/stable/InRelease"
gpg --verify "$DEB_REPO_DIR/dists/stable/Release.gpg" "$DEB_REPO_DIR/dists/stable/Release"

echo "All verifications passed!"