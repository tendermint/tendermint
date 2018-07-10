Version: @VERSION@
Release: @BUILD_NUMBER@

%define __spec_install_post %{nil}
%define debug_package       %{nil}
%define __os_install_post   %{nil}

Name: @PACKAGE_NAME@
Summary: @PACKAGE_SUMMARY@
License: Apache 2.0
URL: @PACKAGE_URL@
Packager: Greg Szabo
Requires: tendermint >= 0.11.0
@PACKAGE_ADDITIONAL_HEADER@

%description
@PACKAGE_DESCRIPTION@

%pre
if ! %{__grep} -q '^%{name}:' /etc/passwd ; then
  useradd -r -b %{_sysconfdir} %{name}
  mkdir -p %{_sysconfdir}/%{name}
  chmod 755 %{_sysconfdir}/%{name}
  chown %{name}.%{name} %{_sysconfdir}/%{name}
fi

%prep
# Nothing to do here. - It is done in the Makefile.

%build
# Nothing to do here.

%install
cd %{name}-%{version}-%{release}
%{__cp} -a * %{buildroot}

%post
sudo -Hu %{name} tendermint init --home %{_sysconfdir}/%{name}
sudo -Hu %{name} %{name} --datadir %{_sysconfdir}/%{name} init %{_sysconfdir}/%{name}/genesis.json

systemctl daemon-reload

%preun
systemctl stop %{name} 2> /dev/null || :
systemctl stop %{name}-service 2> /dev/null || :

%postun
systemctl daemon-reload

%files
%attr(0755, %{name}, %{name}) %dir %{_sysconfdir}/%{name}
%config(noreplace) %attr(0644, %{name}, %{name}) %{_sysconfdir}/%{name}/genesis.json
%attr(0755, %{name}, %{name}) %dir %{_sysconfdir}/%{name}/keystore
%attr(0644, %{name}, %{name}) %{_sysconfdir}/%{name}/keystore/*
%{_bindir}/*
%{_sysconfdir}/systemd/system/*
%{_sysconfdir}/systemd/system-preset/*
%dir %{_defaultlicensedir}/%{name}
%doc %{_defaultlicensedir}/%{name}/LICENSE

