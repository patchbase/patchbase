Name:           patchbase-agent
Version:        0.1.0
Release:        1
Summary:        PatchBase Agent
License:        Apache-2.0
Source0:        @@ARCHIVE_PATH@@

%{!?_bindir:%global _bindir /usr/bin}
%{!?_unitdir:%global _unitdir /usr/lib/systemd/system}
%{!?_sysconfdir:%global _sysconfdir /etc}
%{!?_sharedstatedir:%global _sharedstatedir /var/lib}

Provides:       %{name} = %{version}

%define __spec_install_post %{nil}
%define _unpackaged_files_terminate_build 0
%global _build_id_links none

%description
PatchBase agent collects system package snapshots and reports them to the server.

%global debug_package %{nil}

%autosetup

%build
[ -d bazel-out ] || ln -sT ../bazel-out bazel-out
tar xvf @@ARCHIVE_PATH@@

%install
install -Dpm 755 patchbase-agent %{buildroot}%{_bindir}/patchbase-agent
install -Dpm 644 patchbase-agent.service %{buildroot}%{_unitdir}/patchbase-agent.service
install -Dpm 644 patchbase-agent.timer %{buildroot}%{_unitdir}/patchbase-agent.timer
install -d %{buildroot}%{_sysconfdir}/patchbase-agent
install -d %{buildroot}%{_sharedstatedir}/patchbase-agent

%post
if [ $1 -eq 1 ] ; then
    # Initial installation
    systemctl preset patchbase-agent.service patchbase-agent.timer >/dev/null 2>&1 || :
fi

%preun
if [ $1 -eq 0 ] ; then
    # Package removal
    systemctl --no-reload disable --now patchbase-agent.service patchbase-agent.timer > /dev/null 2>&1 || :
fi

%postun
systemctl daemon-reload >/dev/null 2>&1 || :
if [ $1 -ge 1 ] ; then
    # Package upgrade
    systemctl try-restart patchbase-agent.service patchbase-agent.timer >/dev/null 2>&1 || :
fi

%files
%{_bindir}/patchbase-agent
%{_unitdir}/patchbase-agent.service
%{_unitdir}/patchbase-agent.timer
%dir %attr(0755, root, root) %{_sysconfdir}/patchbase-agent
%dir %attr(0755, root, root) %{_sharedstatedir}/patchbase-agent
