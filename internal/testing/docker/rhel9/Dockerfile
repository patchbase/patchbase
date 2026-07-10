# Pinned Red Hat UBI 9.3 image for patchbase integration tests.
#
# Uses the official Red Hat Universal Base Image (UBI), which is freely
# redistributable and publicly available from registry.access.redhat.com
# without authentication. UBI is the real RHEL userspace: same
# /etc/os-release (ID=rhel), same package signing (vendor "Red Hat, Inc."),
# same repo structure as full RHEL subscriptions.
#
# Package versions come from the UBI repos, which are version-stable per
# image tag. The base image tag (ubi9/ubi:9.3) is the pin; we additionally
# pin the exact NEVRAs of openssh-server, nginx and openssl-libs so a
# future repo metadata refresh doesn't silently change the assertions.
#
# The authorized_keys file is NOT baked into the image; the test harness
# injects it at runtime via WriteAuthorizedKey. The test SSH keypair
# (used by tests that connect via SSH directly) lives at
# internal/testing/docker/fixtures/ — see fixtures/regenerate.sh.
FROM registry.access.redhat.com/ubi9/ubi:9.3

# Install openssh-server (shared by agent + SSH-pull tests), nginx at a
# pinned known-vulnerable version (1.20.1 from the base, non-module package
# set), and openssl-libs at a fixed version for non-trivial package
# coverage. NEVRAs match the UBI 9.3 repos; if dnf fails because the
# version has rolled, bump the version strings. nginx is shipped at
# epoch 2 in RHEL 9.
RUN dnf install -y --best --allowerasing \
      openssh-server-0:9.9p1-7.el9_8 \
      nginx-2:1.20.1-28.el9_8.4 \
      openssl-libs-1:3.5.5-4.el9_8 \
      curl \
 && dnf clean all

# Configure sshd: allow root login with key auth and generate host keys
# (UBI's openssh-server package does not create them in postinst the way
# Debian's does). The authorized_keys file is injected at runtime by the
# test harness (WriteAuthorizedKey), not baked into the image, so there
# is a single source of truth for the test keypair.
RUN mkdir -p /run/sshd /root/.ssh \
 && chmod 700 /root/.ssh \
 && ssh-keygen -A \
 && sed -i 's/^#*PermitRootLogin.*/PermitRootLogin prohibit-password/' /etc/ssh/sshd_config \
 && sed -i 's/^#*PubkeyAuthentication.*/PubkeyAuthentication yes/' /etc/ssh/sshd_config

# Fixed machine-id for deterministic enrollment identity.
RUN echo "patchbase-test-rhel9-machine-id-0001" > /etc/machine-id

# sshd listens on port 22; the test harness starts sshd manually via
# `docker exec` (no systemd in this image).
EXPOSE 22

# Default command keeps the container alive; tests start sshd via docker exec.
CMD ["sleep", "infinity"]