# Pinned Red Hat UBI 10.0 image for patchbase integration tests.
#
# Uses the official Red Hat Universal Base Image (UBI) for RHEL 10, the
# new major release. RHEL 10 ships a newer kernel, newer dnf (4.20.0 at
# time of writing), and a different repo-id naming scheme
# (ubi-10-for-$basearch-baseos-rpms vs ubi-9-baseos-rpms). Testing it
# separately catches format drift that the 9.x distros (Rocky/Alma/RHEL 9)
# won't surface.
#
# Package versions come from the UBI repos, which are version-stable per
# image tag. The base image tag (ubi10/ubi:10.0) is the pin; we
# additionally pin the exact NEVRAs of openssh-server, nginx and
# openssl-libs so a future repo metadata refresh doesn't silently change
# the assertions.
#
# The authorized_keys file is NOT baked into the image; the test harness
# injects it at runtime via WriteAuthorizedKey. The test SSH keypair
# (used by tests that connect via SSH directly) lives at
# internal/testing/docker/fixtures/ — see fixtures/regenerate.sh.
FROM registry.access.redhat.com/ubi10/ubi:10.0

# Install openssh-server (shared by agent + SSH-pull tests), nginx at a
# pinned version (1.26.3 from the AppStream default module stream in RHEL
# 10 — unlike RHEL 9 the base 1.20.1 package is no longer shipped), and
# openssl-libs at a fixed version for non-trivial package coverage. NEVRAs
# match the UBI 10.0 repos; if dnf fails because the version has rolled,
# bump the version strings. nginx is shipped at epoch 2 in RHEL 10.
RUN dnf install -y --best --allowerasing \
      openssh-server-0:9.9p1-23.el10_2 \
      nginx-2:1.26.3-6.el10_2.4 \
      openssl-libs-1:3.5.5-4.el10_2 \
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
RUN echo "patchbase-test-rhel10-machine-id-0001" > /etc/machine-id

# sshd listens on port 22; the test harness starts sshd manually via
# `docker exec` (no systemd in this image).
EXPOSE 22

# Default command keeps the container alive; tests start sshd via docker exec.
CMD ["sleep", "infinity"]