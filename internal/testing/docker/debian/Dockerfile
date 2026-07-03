# Pinned Debian 12 (bookworm) image for patchbase integration tests.
#
# Package versions are pinned via snapshot.debian.org so test results are
# reproducible regardless of when CI runs. The fixed snapshot date is the
# single knob to adjust when bumping the image.
#
# The authorized_keys file is NOT baked into the image; the test harness
# injects it at runtime via WriteAuthorizedKey. The test SSH keypair
# (used by tests that connect via SSH directly) lives at
# internal/testing/docker/fixtures/ — see fixtures/regenerate.sh.
FROM debian:12

ENV DEBIAN_FRONTEND=noninteractive

# Pin apt to a snapshot.debian.org archive timestamp so package versions are
# reproducible. Bump the URL below (and the package versions) to roll the
# image forward. The debian:12 base image uses the deb822 format in
# sources.list.d/debian.sources, so we rewrite that file rather than
# /etc/apt/sources.list.
RUN rm -f /etc/apt/sources.list /etc/apt/sources.list.d/*.sources \
 && printf 'Types: deb\nURIs: http://snapshot.debian.org/archive/debian/20260701T000000Z\nSuites: bookworm\nComponents: main\nSigned-By: /usr/share/keyrings/debian-archive-keyring.gpg\n' > /etc/apt/sources.list.d/patchbase.sources

# Install openssh-server (shared by agent + SSH-pull tests), nginx at a pinned
# known-vulnerable version, and a fixed-version libssl3 for non-trivial package
# coverage. Versions match the snapshot date above.
RUN apt-get update \
 && apt-get install -y --no-install-recommends \
      openssh-server=1:9.2p1-2+deb12u10 \
      nginx=1.22.1-9+deb12u6 \
      libssl3=3.0.20-1~deb12u1 \
      curl \
 && rm -rf /var/lib/apt/lists/*

# Configure sshd: allow root login with key auth. The authorized_keys
# file is injected at runtime by the test harness (WriteAuthorizedKey),
# not baked into the image, so there is a single source of truth for
# the test keypair.
RUN mkdir -p /run/sshd /root/.ssh \
 && chmod 700 /root/.ssh \
 && sed -i 's/^#*PermitRootLogin.*/PermitRootLogin prohibit-password/' /etc/ssh/sshd_config \
 && sed -i 's/^#*PubkeyAuthentication.*/PubkeyAuthentication yes/' /etc/ssh/sshd_config

# Fixed machine-id for deterministic enrollment identity.
RUN echo "patchbase-test-debian-machine-id-0001" > /etc/machine-id

# sshd listens on port 22; the test harness starts sshd manually via
# `docker exec` (no systemd in this image).
EXPOSE 22

# Default command keeps the container alive; tests start sshd via docker exec.
CMD ["sleep", "infinity"]
