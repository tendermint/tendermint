
%define __spec_install_post %{nil}
%define debug_package       %{nil}
%define __os_install_post   %{nil}

Name: trackomatron
Summary: Trackomatron - Track invoices on the blockchain
License: Apache 2.0
URL: https://tendermint.com/
Packager: Greg Szabo
Requires: tendermint >= 0.10.0
Requires(pre): /sbin/useradd

%description
This software is intended to create a space to easily send invoices between and within institutions. Firstly, the commands of trackmatron are separated into two broad categories: submitting information to the blockchain (transactions), and retrieving information from the blockchain (query).

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

%{__cp} $GOPATH/bin/tracko $GOPATH/bin/trackocli .%{_bindir}
%{__cp} $GOPATH/src/github.com/tendermint/%{name}/LICENSE .%{_defaultlicensedir}/%{name}

cp -r %{_topdir}/extrafiles/%{name}/* ./

%{__chmod} -Rf a+rX,u+w,g-w,o-w .

%build
# Nothing to do here.

%install
cd %{name}-%{version}
%{__cp} -a * %{buildroot}

%post
sudo -Hu %{name} tracko init --home %{_sysconfdir}/%{name} 2B24DEE2364762300168DF19B6C18BCE2D399EA2
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

