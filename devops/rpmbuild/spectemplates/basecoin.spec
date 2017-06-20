
%define __spec_install_post %{nil}
%define debug_package       %{nil}
%define __os_install_post   %{nil}

Name: basecoin
Summary: basecoin is a Proof-of-Stake cryptocurrency and framework
License: Apache 2.0
URL: https://tendermint.com/
Packager: Greg Szabo
Requires: tendermint >= 0.10.0
Provides: basecli
Requires(pre): /sbin/useradd

%description
Basecoin is an ABCI application designed to be used with the Tendermint consensus engine to form a Proof-of-Stake cryptocurrency. It also provides a general purpose framework for extending the feature-set of the cryptocurrency by implementing plugins.

%pre
if ! %{__grep} -q '^%{name}:' /etc/passwd ; then
  useradd -k /dev/null -r -m -b %{_sysconfdir} %{name}
fi

%prep
test -d "$GOPATH" || echo "GOPATH not set"
test -d "$GOPATH"

%{__mkdir_p} %{name}-%{version}
cd %{name}-%{version}

%{__mkdir_p} .%{_bindir} .%{_defaultlicensedir}/%{name} .%{_sysconfdir}/%{name}/tendermint

%{__cp} $GOPATH/bin/%{name} $GOPATH/bin/basecli .%{_bindir}
%{__cp} $GOPATH/src/github.com/tendermint/%{name}/LICENSE .%{_defaultlicensedir}/%{name}

cp -r %{_topdir}/extrafiles/* ./

%{__chmod} -Rf a+rX,u+w,g-w,o-w .

%build
# Nothing to do here.

%install
cd %{name}-%{version}
%{__cp} -a * %{buildroot}

%post
sudo -Hu %{name} basecoin init --home %{_sysconfdir}/%{name}
#The above command generates a genesis.json file that contains validators. This is wrong, the validator part should be empty. https://github.com/tendermint/basecoin/issues/124
sudo -Hu %{name} tendermint init --home %{_sysconfdir}/%{name}/tendermint
#The above command might need some kind of additional option in the future. https://github.com/tendermint/tendermint/issues/542

#Temporary until https://github.com/tendermint/basecoin/issues/123
rm -f %{_sysconfdir}/%{name}/key.json
rm -f %{_sysconfdir}/%{name}/key2.json

systemctl daemon-reload

%preun
systemctl stop %{name} 2> /dev/null || :
systemctl stop %{name}-service 2> /dev/null || :

%postun
systemctl daemon-reload

%files
%attr(0755, %{name}, %{name}) %dir %{_sysconfdir}/%{name}
%attr(0755, %{name}, %{name}) %dir %{_sysconfdir}/%{name}/tendermint
%{_bindir}/*
%{_sysconfdir}/systemd/system/*
%{_sysconfdir}/systemd/system-preset/*
%dir %{_datadir}/%{name}
%{_datadir}/%{name}/*
%dir %{_defaultlicensedir}/%{name}
%doc %{_defaultlicensedir}/%{name}/LICENSE

