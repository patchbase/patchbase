Name:           patchbase-server
Version:        0.1.0
Release:        1
Summary:        PatchBase Server
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
PatchBase server is the backend service for PatchBase.

%global debug_package %{nil}

%autosetup

%build
[ -d bazel-out ] || ln -sT ../bazel-out bazel-out
tar xvf @@ARCHIVE_PATH@@

%install
install -Dpm 755 patchbase-server %{buildroot}%{_bindir}/patchbase-server
install -Dpm 644 config.example.yaml %{buildroot}%{_sysconfdir}/patchbase-server/config.example.yaml
install -Dpm 644 patchbase-server.service %{buildroot}%{_unitdir}/patchbase-server.service
install -d %{buildroot}%{_sysconfdir}/patchbase-server
install -d %{buildroot}%{_sharedstatedir}/patchbase-server

%post
%systemd_post patchbase-server.service

%preun
%systemd_preun patchbase-server.service

%files
%{_bindir}/patchbase-server
%{_unitdir}/patchbase-server.service
%dir %attr(0755, root, root) %{_sysconfdir}/patchbase-server
%{_sysconfdir}/patchbase-server/config.example.yaml
%dir %attr(0755, root, root) %{_sharedstatedir}/patchbase-server
