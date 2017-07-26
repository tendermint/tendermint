Version: @VERSION@
Release: @BUILD_NUMBER@

%define __spec_install_post %{nil}
%define debug_package       %{nil}
%define __os_install_post   %{nil}

Name: ethermint
Summary: ethermint enables ethereum as an ABCI application on tendermint and the COSMOS hub
License: Apache 2.0
URL: https://tendermint.com/
Packager: Greg Szabo
Requires: tendermint >= 0.10.0
Requires(pre): /sbin/useradd

%description
Ethermint enables ethereum to run as an ABCI application on tendermint and the COSMOS hub. This application allows you to get all the benefits of ethereum without having to run your own miners.

%pre
if ! %{__grep} -q '^%{name}:' /etc/passwd ; then
  useradd -k /dev/null -r -m -b %{_sysconfdir} %{name}
  chmod 755 %{_sysconfdir}/%{name}
fi

%prep
# Nothing to do here. - It is done in the Makefile.

%build
# Nothing to do here.

%install
cd %{name}-%{version}-%{release}
%{__cp} -a * %{buildroot}

%post
sudo -Hu %{name} %{_bindir}/%{name} --datadir %{_sysconfdir}/%{name} init %{_sysconfdir}/%{name}/genesis.json
sudo -Hu %{name} tendermint init --home %{_sysconfdir}/%{name}/tendermint

chmod 755 %{_sysconfdir}/%{name}/tendermint

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
%ghost %attr(0755, %{name}, %{name}) %dir %{_sysconfdir}/%{name}/tendermint
%{_bindir}/*
%{_sysconfdir}/systemd/system/*
%{_sysconfdir}/systemd/system-preset/*
%dir %{_defaultlicensedir}/%{name}
%doc %{_defaultlicensedir}/%{name}/LICENSE

