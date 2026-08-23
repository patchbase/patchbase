# Pinned Ubuntu 24.04 LTS (Noble Numbat) image for patchbase integration tests.
#
# Package versions are pinned via snapshot.ubuntu.com so test results are
# reproducible regardless of when CI runs. The fixed snapshot timestamp is the
# single knob to adjust when bumping the image.
#
# The authorized_keys file is NOT baked into the image; the test harness
# injects it at runtime via WriteAuthorizedKey. The test SSH keypair
# (used by tests that connect via SSH directly) lives at
# internal/testing/docker/fixtures/ — see fixtures/regenerate.sh.
FROM ubuntu:24.04

ENV DEBIAN_FRONTEND=noninteractive

# Pin apt to a snapshot.ubuntu.com archive timestamp so package versions are
# reproducible regardless of when CI runs. The fixed snapshot timestamp is the
# single knob to adjust when bumping the image.
#
# The snapshot endpoint redirects to HTTPS; disable peer verification
# temporarily during package bootstrap before ca-certificates is installed.
RUN rm -f /etc/apt/sources.list /etc/apt/sources.list.d/*.sources \
 && printf 'Acquire::https::Verify-Peer "false";\nAcquire::https::Verify-Host "false";\n' > /etc/apt/apt.conf.d/99ssl-nocheck \
 && printf 'Types: deb\nURIs: https://snapshot.ubuntu.com/ubuntu/20260701T000000Z\nSuites: noble noble-updates\nComponents: main universe\nSigned-By: /usr/share/keyrings/ubuntu-archive-keyring.gpg\n' > /etc/apt/sources.list.d/patchbase.sources

# Install openssh-server (shared by agent + SSH-pull tests), nginx at a pinned
# known-vulnerable version, and a fixed-version libssl3t64 for non-trivial
# package coverage. Versions match the snapshot date above.
#
# Note: on noble the OpenSSL library package is `libssl3t64` (the t64
# transition package); on older releases it was `libssl3`. Adjust the
# name when bumping the base image.
RUN apt-get update \
 && apt-get install -y --no-install-recommends \
      ca-certificates \
      openssh-server \
      nginx=1.24.0-2ubuntu7.13 \
      curl \
 && rm -f /etc/apt/apt.conf.d/99ssl-nocheck \
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
RUN echo "patchbase-test-ubuntu-machine-id-0001" > /etc/machine-id

# sshd listens on port 22; the test harness starts sshd manually via
# `docker exec` (no systemd in this image).
EXPOSE 22

# Default command keeps the container alive; tests start sshd via docker exec.
CMD ["sleep", "infinity"]
