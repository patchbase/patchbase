# Pinned Ubuntu 24.04 LTS (Noble Numbat) image for patchbase integration tests.
#
# Ubuntu does not have a snapshot.debian.org equivalent. The base image tag
# `ubuntu:24.04` is itself a point-release pin (Noble Numbat LTS, 5-year
# support); for reproducible package versions we additionally pin the
# exact versions of openssh-server, nginx and libssl3t64 in the install
# step. The single knob to roll this image forward is the package version
# string below — bump them when the archive drifts.
#
# The authorized_keys file is NOT baked into the image; the test harness
# injects it at runtime via WriteAuthorizedKey. The test SSH keypair
# (used by tests that connect via SSH directly) lives at
# internal/testing/docker/fixtures/ — see fixtures/regenerate.sh.
FROM ubuntu:24.04

ENV DEBIAN_FRONTEND=noninteractive

# Install openssh-server (shared by agent + SSH-pull tests), nginx at a pinned
# known-vulnerable version, and a fixed-version libssl3t64 for non-trivial
# package coverage. Versions are pinned to the archive as of this commit;
# if apt-get update fails because the version is no longer in the
# archive, switch to http://old-releases.ubuntu.com/ubuntu/ for the
# point release.
#
# Note: on noble the OpenSSL library package is `libssl3t64` (the t64
# transition package); on older releases it was `libssl3`. Adjust the
# name when bumping the base image.
RUN apt-get update \
 && apt-get install -y --no-install-recommends \
      openssh-server=1:9.6p1-3ubuntu13.16 \
      nginx=1.24.0-2ubuntu7.13 \
      libssl3t64=3.0.13-0ubuntu3.11 \
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
RUN echo "patchbase-test-ubuntu-machine-id-0001" > /etc/machine-id

# sshd listens on port 22; the test harness starts sshd manually via
# `docker exec` (no systemd in this image).
EXPOSE 22

# Default command keeps the container alive; tests start sshd via docker exec.
CMD ["sleep", "infinity"]
