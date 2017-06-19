
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

%{__mkdir_p} .%{_bindir} .%{_defaultlicensedir}/%{name} .%{_sysconfdir}/%{name}/tendermint .%{_datadir}/%{name} .%{_sysconfdir}/systemd/system .%{_sysconfdir}/systemd/system-preset

%{__cp} $GOPATH/bin/%{name} $GOPATH/bin/basecli .%{_bindir}
%{__cp} $GOPATH/src/github.com/tendermint/%{name}/LICENSE .%{_defaultlicensedir}/%{name}
%{__cp} %{_topdir}/extrafiles/%{name}/genesis.json .%{_sysconfdir}/%{name}/genesis.json
%{__cp} %{_topdir}/extrafiles/%{name}/tendermint-config.toml .%{_sysconfdir}/%{name}/tendermint/config.toml
%{__cp} %{_topdir}/extrafiles/%{name}/%{name}.service .%{_sysconfdir}/systemd/system/%{name}.service
%{__cp} %{_topdir}/extrafiles/%{name}/%{name}-server.service .%{_sysconfdir}/systemd/system/%{name}-server.service
%{__cp} %{_topdir}/extrafiles/%{name}/50-%{name}.preset .%{_sysconfdir}/systemd/system-preset/50-%{name}.preset
%{__cp} %{_topdir}/extrafiles/%{name}/key.json .%{_datadir}/%{name}/key.json
%{__cp} %{_topdir}/extrafiles/%{name}/key2.json .%{_datadir}/%{name}/key2.json

%{__chmod} -Rf a+rX,u+w,g-w,o-w .

%build
# Nothing to do here.

%install
cd %{name}-%{version}
%{__cp} -a * %{buildroot}

%post
test ! -f %{_sysconfdir}/%{name}/priv_validator.json && tendermint gen_validator > %{_sysconfdir}/%{name}/priv_validator.json && %{__chmod} 0400 %{_sysconfdir}/%{name}/priv_validator.json && %{__chown} %{name}.%{name} %{_sysconfdir}/%{name}/priv_validator.json
test ! -f %{_sysconfdir}/%{name}/tendermint/priv_validator.json && tendermint gen_validator > %{_sysconfdir}/%{name}/tendermint/priv_validator.json && %{__chmod} 0400 %{_sysconfdir}/%{name}/tendermint/priv_validator.json && %{__chown} %{name}.%{name} %{_sysconfdir}/%{name}/tendermint/priv_validator.json
tendermint_pubkey="`tendermint show_validator --home /etc/ethermint/tendermint --log_level error`"
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
#%ghost %attr(0400, %{name}, %{name}) %{_sysconfdir}/%{name}/priv_validator.json
%config(noreplace) %attr(0644, %{name}, %{name}) %{_sysconfdir}/%{name}/genesis.json
%attr(0755, %{name}, %{name}) %dir %{_sysconfdir}/%{name}/tendermint
#%ghost %attr(0400, %{name}, %{name}) %{_sysconfdir}/%{name}/tendermint/priv_validator.json
%config(noreplace) %attr(0644, %{name}, %{name}) %{_sysconfdir}/%{name}/tendermint/config.toml
#%ghost %attr(0644, %{name}, %{name}) %{_sysconfdir}/%{name}/tendermint/genesis.json
%{_bindir}/*
%{_sysconfdir}/systemd/system/*
%{_sysconfdir}/systemd/system-preset/*
%dir %{_datadir}/%{name}
%{_datadir}/%{name}/*
%dir %{_defaultlicensedir}/%{name}
%doc %{_defaultlicensedir}/%{name}/LICENSE

