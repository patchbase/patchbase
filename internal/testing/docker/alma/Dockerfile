# Pinned AlmaLinux 9.7 (Moss Jungle Cat) image for patchbase integration tests.
#
# Package versions are pinned via the AlmaLinux vault mirror
# (https://repo.almalinux.org/vault/9.7/...) so test results are
# reproducible regardless of when CI runs. The fixed base tag + vault URL
# pair is the single knob to adjust when bumping the image.
#
# The authorized_keys file is NOT baked into the image; the test harness
# injects it at runtime via WriteAuthorizedKey. The test SSH keypair
# (used by tests that connect via SSH directly) lives at
# internal/testing/docker/fixtures/ — see fixtures/regenerate.sh.
FROM almalinux:9.7

# Pin the enabled repos to the 9.7 vault BaseOS/AppStream trees so package
# versions are frozen to the 9.7 point release. The default AlmaLinux repos
# use mirrorlist which resolve to whatever the mirror has most recently
# (e.g. 9.7 + security updates), which would drift. We replace them with
# explicit baseurl entries pointing at the frozen 9.7 vault directory.
RUN rm -f /etc/yum.repos.d/*.repo \
 && printf '[baseos]\nname=AlmaLinux 9.7 - BaseOS\nbaseurl=https://repo.almalinux.org/vault/9.7/BaseOS/x86_64/os/\ngpgcheck=1\nenabled=1\ngpgkey=file:///etc/pki/rpm-gpg/RPM-GPG-KEY-AlmaLinux-9\nmetadata_expire=never\n' > /etc/yum.repos.d/almalinux-baseos.repo \
 && printf '[appstream]\nname=AlmaLinux 9.7 - AppStream\nbaseurl=https://repo.almalinux.org/vault/9.7/AppStream/x86_64/os/\ngpgcheck=1\nenabled=1\ngpgkey=file:///etc/pki/rpm-gpg/RPM-GPG-KEY-AlmaLinux-9\nmetadata_expire=never\n' > /etc/yum.repos.d/almalinux-appstream.repo

# Install openssh-server (shared by agent + SSH-pull tests), nginx at a pinned
# known-vulnerable version (1.20.1 from the base, non-module package set in
# the 9.7 vault — the AppStream module variants carry awkward
# `module_el9.7.0+...` release strings that are painful to pin through dnf's
# module enablement dance), and openssl-libs at a fixed version for
# non-trivial package coverage. NEVRAs match the 9.7 vault above; if dnf
# fails because the version has rolled, bump the version strings.
#
# nginx is shipped at epoch 2 in AlmaLinux 9. The `.alma.1` suffix on the
# release string is Alma's rebuild marker (vs Rocky's `.rocky.0.1`).
RUN dnf install -y --best --allowerasing \
      openssh-server-0:8.7p1-46.el9.alma.1 \
      nginx-2:1.20.1-24.el9_7.1.alma.1 \
      openssl-libs-1:3.5.1-4.el9_7 \
      curl \
 && dnf clean all

# Configure sshd: allow root login with key auth and generate host keys
# (AlmaLinux's openssh-server package does not create them in postinst the
# way Debian's does). The authorized_keys file is injected at runtime by
# the test harness (WriteAuthorizedKey), not baked into the image, so there
# is a single source of truth for the test keypair.
RUN mkdir -p /run/sshd /root/.ssh \
 && chmod 700 /root/.ssh \
 && ssh-keygen -A \
 && sed -i 's/^#*PermitRootLogin.*/PermitRootLogin prohibit-password/' /etc/ssh/sshd_config \
 && sed -i 's/^#*PubkeyAuthentication.*/PubkeyAuthentication yes/' /etc/ssh/sshd_config

# Fixed machine-id for deterministic enrollment identity.
RUN echo "patchbase-test-alma-machine-id-0001" > /etc/machine-id

# sshd listens on port 22; the test harness starts sshd manually via
# `docker exec` (no systemd in this image).
EXPOSE 22

# Default command keeps the container alive; tests start sshd via docker exec.
CMD ["sleep", "infinity"]