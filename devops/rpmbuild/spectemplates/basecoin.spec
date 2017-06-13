
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
Requires(post): %{__python}

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
#iso8601date=`date -u +"%Y-%m-%dT%H:%M:%SZ"`
%{__cat} << EOF > .%{_sysconfdir}/%{name}/genesis.json
{
  "genesis_time": "2017-06-10T03:37:03Z",
  "chain_id": "my_chain_id",
  "validators": [],
  "app_hash": "",
  "app_options": {
    "accounts": [{
      "name": "dev_money",
      "address": "1B1BE55F969F54064628A63B9559E7C21C925165",
      "pub_key": {
        "type": "ed25519",
        "data": "619D3678599971ED29C7529DDD4DA537B97129893598A17C82E3AC9A8BA95279"
      },
      "coins": [
        {
          "denom": "mycoin",
          "amount": 9007199254740992
        }
      ]
    }]
  }
}
EOF
%{__cat} << EOF > .%{_sysconfdir}/%{name}/tendermint/config.toml
# This is a TOML config file.
# For more information, see https://github.com/toml-lang/toml

proxy_app = "tcp://127.0.0.1:36658"
moniker = ""
fast_sync = true
db_backend = "leveldb"
log_level = "debug"

[rpc]
laddr = "tcp://0.0.0.0:36657"

[p2p]
laddr = "tcp://0.0.0.0:36656"
seeds = ""
EOF
%{__cat} << EOF > .%{_sysconfdir}/systemd/system/%{name}.service
[Unit]
Description=Basecoin
#propagates activation, deactivation and activation fails. 
Requires=network-online.target
#propagates activation, deactivation, activation fails and stops
BindTo=%{name}-server.service
#propagates stop and restart (one-way)
PartOf=%{name}-server.service
#order
Before=%{name}-server.service
After=network-online.target
#propagates reload
PropagatesReloadTo=%{name}-server.service
ReloadPropagatedFrom=%{name}-server.service

[Service]
Environment="BCHOME=%{_sysconfdir}/%{name}"
Restart=on-failure
User=%{name}
Group=%{name}
PermissionsStartOnly=true
ExecStart=%{_bindir}/%{name} start --without-tendermint
ExecReload=/bin/kill -HUP \$MAINPID
KillSignal=SIGTERM

[Install]
WantedBy=multi-user.target
Also=%{name}-server.service
EOF
%{__cat} << EOF > .%{_sysconfdir}/systemd/system/%{name}-server.service
[Unit]
Description=Basecoin server
Requires=network-online.target
BindTo=%{name}.service
PartOf=%{name}.service
After=network-online.target %{name}.service
PropagatesReloadTo=%{name}.service
ReloadPropagatedFrom=%{name}.service

[Service]
Environment="TMHOME=%{_sysconfdir}/%{name}/tendermint"
Restart=on-failure
User=%{name}
Group=%{name}
PermissionsStartOnly=true
ExecStart=%{_bindir}/tendermint node
ExecReload=/bin/kill -HUP \$MAINPID
KillSignal=SIGTERM

[Install]
WantedBy=multi-user.target %{name}.service
Also=%{name}.service
EOF
%{__cat} << EOF > .%{_sysconfdir}/systemd/system-preset/50-%{name}.preset
disable %{name}.service
disable %{name}-server.service
EOF
%{__cat} << EOF > .%{_datadir}/%{name}/key.json
{
  "address": "1B1BE55F969F54064628A63B9559E7C21C925165",
  "priv_key": {
    "type": "ed25519",
    "data": "C70D6934B4F55F1B7BC33B56B9CA8A2061384AFC19E91E44B40C4BBA182953D1619D3678599971ED29C7529DDD4DA537B97129893598A17C82E3AC9A8BA95279"
  },
  "pub_key": {
    "type": "ed25519",
    "data": "619D3678599971ED29C7529DDD4DA537B97129893598A17C82E3AC9A8BA95279"
  }
}
EOF
%{__cat} << EOF > .%{_datadir}/%{name}/key2.json
{
  "address": "1DA7C74F9C219229FD54CC9F7386D5A3839F0090",
  "priv_key": {
    "type": "ed25519",
    "data": "34BAE9E65CE8245FAD035A0E3EED9401BDE8785FFB3199ACCF8F5B5DDF7486A8352195DA90CB0B90C24295B90AEBA25A5A71BC61BAB2FE2387241D439698B7B8"
  },
  "pub_key": {
    "type": "ed25519",
    "data": "352195DA90CB0B90C24295B90AEBA25A5A71BC61BAB2FE2387241D439698B7B8"
  }
}
EOF
%{__cat} << EOF > .%{_datadir}/%{name}/tendermint-genesis-validator.json
{
  "genesis_time":"2017-06-10T03:37:03Z",
  "chain_id":"my_testchain_id",
  "validators":
  [
    {
      "pub_key":{"type":"ed25519","data":"F651E966D30CEA413839D63D9EC37455DF1FB2DDC409A76555B0CD0B186723E4"},
      "amount":10,
      "name":"my_testchain_node"
    }
  ],
  "app_hash":"",
  "app_options": {}
}
EOF
%{__cat} << EOF > .%{_datadir}/%{name}/basecoin-genesis-devmoney.json
{
  "genesis_time":"2017-06-10T03:37:03Z",
  "chain_id":"my_testchain_id",
  "validators": [],
  "app_hash":"",
  "app_options": {
    "accounts": [{
      "pub_key": {
        "type": "ed25519",
        "data": "619D3678599971ED29C7529DDD4DA537B97129893598A17C82E3AC9A8BA95279"
      },
      "coins": [
        {
          "denom": "mycoin",
          "amount": 9007199254740992
        }
      ]
    }]
  }
}
EOF
%{__cat} << EOF > .%{_datadir}/%{name}/priv_validator-example.json
{"address":"AF5886732763B3A38861FC8D2AA2698A156D28BD","pub_key":{"type":"ed25519","data":"F651E966D30CEA413839D63D9EC37455DF1FB2DDC409A76555B0CD0B186723E4"},"last_height":159,"last_round":0,"last_step":3,"last_signature":{"type":"ed25519","data":"654E93AA49B91F48AFEBE172E2C296900339F8849577467AEA9885200D5D9CD4DC225BB099C436A60C832DD2A3C60B6834D9748C0F8E977420BF3696604E2F04"},"last_signbytes":"7B22636861696E5F6964223A22677265677465737431222C22766F7465223A7B22626C6F636B5F6964223A7B2268617368223A2246333144343534433043343939334237333641323332393738443738333645453846384642433031222C227061727473223A7B2268617368223A2246304430443241333933443241444530323739303244304438333643384532443938333231304234222C22746F74616C223A317D7D2C22686569676874223A3135392C22726F756E64223A302C2274797065223A327D7D","priv_key":{"type":"ed25519","data":"44BD99EE9FFAEA5CECA7E52EF7476A5C949BC8635BCB906468C7CC3456659809F651E966D30CEA413839D63D9EC37455DF1FB2DDC409A76555B0CD0B186723E4"}}
EOF
%{__chmod} -Rf a+rX,u+w,g-w,o-w .

%build
# Nothing to do here.

%install
cd %{name}-%{version}
%{__cp} -a * %{buildroot}

%post
test ! -f %{_sysconfdir}/%{name}/priv_validator.json && tendermint gen_validator > %{_sysconfdir}/%{name}/priv_validator.json && %{__chmod} 0400 %{_sysconfdir}/%{name}/priv_validator.json && %{__chown} %{name}.%{name} %{_sysconfdir}/%{name}/priv_validator.json
test ! -f %{_sysconfdir}/%{name}/tendermint/priv_validator.json && tendermint gen_validator > %{_sysconfdir}/%{name}/tendermint/priv_validator.json && %{__chmod} 0400 %{_sysconfdir}/%{name}/tendermint/priv_validator.json && %{__chown} %{name}.%{name} %{_sysconfdir}/%{name}/tendermint/priv_validator.json
tendermint_pubkey=`%{__python} -uc "import json ; print json.loads(open('%{_sysconfdir}/%{name}/tendermint/priv_validator.json').read())['pub_key']['data']"`
test ! -f %{_sysconfdir}/%{name}/tendermint/genesis.json && %{__cat} << EOF > %{_sysconfdir}/%{name}/tendermint/genesis.json
{
  "genesis_time": "2017-06-10T03:37:03Z",
  "chain_id": "my_chain_id",
  "validators":
  [
    {
      "pub_key":{"type":"ed25519","data":"$tendermint_pubkey"},
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

%preun
systemctl stop %{name} 2> /dev/null || :
systemctl stop %{name}-service 2> /dev/null || :

%postun
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

