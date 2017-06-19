
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
fi

%prep
test -d "$GOPATH" || echo "GOPATH not set"
test -d "$GOPATH"
%{__mkdir_p} %{name}-%{version}
cd %{name}-%{version}

%{__mkdir_p} .%{_bindir} .%{_defaultlicensedir}/%{name} .%{_sysconfdir}/%{name}/tendermint .%{_sysconfdir}/systemd/system .%{_sysconfdir}/systemd/system-preset

%{__cp} $GOPATH/bin/%{name} .%{_bindir}
%{__cp} $GOPATH/src/github.com/tendermint/%{name}/LICENSE .%{_defaultlicensedir}/%{name}
%{__cp} $GOPATH/src/github.com/tendermint/%{name}/dev/genesis.json .%{_sysconfdir}/%{name}/genesis.json
%{__cp} -r $GOPATH/src/github.com/tendermint/%{name}/dev/keystore .%{_sysconfdir}/%{name}
%{__cp} %{_topdir}/extrafiles/%{name}/tendermint-config.toml .%{_sysconfdir}/%{name}/tendermint/config.toml
%{__cp} %{_topdir}/extrafiles/%{name}/%{name}.service .%{_sysconfdir}/systemd/system/%{name}.service
%{__cp} %{_topdir}/extrafiles/%{name}/%{name}-server.service .%{_sysconfdir}/systemd/system/%{name}-server.service
%{__cp} %{_topdir}/extrafiles/%{name}/50-%{name}.preset .%{_sysconfdir}/systemd/system-preset/50-%{name}.preset

%{__chmod} -Rf a+rX,u+w,g-w,o-w .

%build
# Nothing to do here.

%install
cd %{name}-%{version}
%{__cp} -a * %{buildroot}

%post
%{_bindir}/%{name} --datadir %{_sysconfdir}/%{name} init %{_sysconfdir}/%{name}/genesis.json
test ! -f %{_sysconfdir}/%{name}/tendermint/priv_validator.json && tendermint gen_validator > %{_sysconfdir}/%{name}/tendermint/priv_validator.json && %{__chmod} 0400 %{_sysconfdir}/%{name}/tendermint/priv_validator.json && %{__chown} %{name}.%{name} %{_sysconfdir}/%{name}/tendermint/priv_validator.json
tendermint_pubkey="`tendermint show_validator --home %{_sysconfdir}/%{name}/tendermint --log_level error`"
test ! -f %{_sysconfdir}/%{name}/tendermint/genesis.json && %{__cat} << EOF > %{_sysconfdir}/%{name}/tendermint/genesis.json
{
  "genesis_time": "2017-06-10T03:37:03Z",
  "chain_id": "my_chain_id",
  "validators":
  [
    {
      "pub_key": $tendermint_pubkey,
      "amount":10,
      "name":"my_testchain_node"
    }
  ],
  "app_hash": "",
  "app_options": {}
}
EOF
%{__chown} %{name}.%{name} %{_sysconfdir}/%{name}/tendermint/genesis.json
systemctl daemon-reload
systemctl enable %{name}
if [ -d /etc/%{name}/tendermint/data ]; then
  service %{name} start
fi

%preun
systemctl stop %{name} 2> /dev/null || :
systemctl stop %{name}-service 2> /dev/null || :

%postun
#userdel %{name}
systemctl daemon-reload

%files
%attr(0755, %{name}, %{name}) %dir %{_sysconfdir}/%{name}
%config(noreplace) %attr(0644, %{name}, %{name}) %{_sysconfdir}/%{name}/genesis.json
%attr(0755, %{name}, %{name}) %dir %{_sysconfdir}/%{name}/keystore
%attr(0644, %{name}, %{name}) %{_sysconfdir}/%{name}/keystore/*
%attr(0755, %{name}, %{name}) %dir %{_sysconfdir}/%{name}/tendermint
%config(noreplace) %attr(0644, %{name}, %{name}) %{_sysconfdir}/%{name}/tendermint/config.toml
%{_bindir}/*
%{_sysconfdir}/systemd/system/*
%{_sysconfdir}/systemd/system-preset/*
%dir %{_defaultlicensedir}/%{name}
%doc %{_defaultlicensedir}/%{name}/LICENSE

