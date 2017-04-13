# Ansible role for Tendermint

![Ansible plus Tendermint](img/a_plus_t.png)

* [Requirements](#requirements)
* [Variables](#variables)
* [Handlers](#handlers)
* [Example playbook that configures a Tendermint on Ubuntu](#example-playbook-that-configures-a-tendermint-on-ubuntu)

`ansible-tendermint` is an [ansible](http://www.ansible.com/) role which:

* installs tendermint
* configures tendermint
* configures tendermint service

## Requirements

This role requires Ansible 2.0 or higher.

## Variables

Here is a list of all the default variables for this role, which are also
available in `defaults/main.yml`.

```
tendermint_version: 0.9.0
tendermint_archive: "tendermint_{{tendermint_version}}_linux_amd64.zip"
tendermint_download: "https://s3-us-west-2.amazonaws.com/tendermint/{{tendermint_version}}/{{tendermint_archive}}"
tendermint_download_folder: /tmp

tendermint_user: tendermint
tendermint_group: tendermint

# Upstart start/stop conditions can vary by distribution and environment
tendermint_upstart_start_on: start on runlevel [345]
tendermint_upstart_stop_on: stop on runlevel [!345]
tendermint_manage_service: true
tendermint_use_upstart: true
tendermint_use_systemd: false
tendermint_upstart_template: "tendermint.conf.j2"
tendermint_systemd_template: "tendermint.systemd.j2"

tendermint_home: /opt/tendermint
tendermint_node_port: 46656
tendermint_rpc_port: 46657
tendermint_proxy_app: "tcp://127.0.0.1:46658"
tendermint_node_laddr: "tcp://0.0.0.0:{{tendermint_node_port}}"
tendermint_rpc_laddr: "tcp://0.0.0.0:{{tendermint_rpc_port}}"
tendermint_seeds: ""
tendermint_fast_sync: true
tendermint_db_backend: leveldb
tendermint_log_level: notice
tendermint_genesis_file: "{{tendermint_home}}/genesis.json"
tendermint_abci: socket
tendermint_skip_upnp: false
tendermint_addrbook_file: "{{tendermint_home}}/addrbook.json"
tendermint_addrbook_strict: true
tendermint_pex_reactor: false
tendermint_priv_validator_file: "{{tendermint_home}}/priv_validator.json"
tendermint_db_dir: "{{tendermint_home}}/data"
tendermint_grpc_laddr: ""
tendermint_prof_laddr: ""
tendermint_cs_wal_file: "{{tendermint_db_dir}}/cs.wal/wal"
tendermint_cs_wal_light: false
tendermint_filter_peers: false
tendermint_block_size: 10000
tendermint_block_part_size: 65536
tendermint_disable_data_hash: false
# all timeouts are in milliseconds
tendermint_timeout_propose: 3000
tendermint_timeout_propose_delta: 500
tendermint_timeout_prevote: 1000
tendermint_timeout_prevote_delta: 500
tendermint_timeout_precommit: 1000
tendermint_timeout_precommit_delta: 500
tendermint_timeout_commit: 1000
tendermint_skip_timeout_commit: false
tendermint_mempool_recheck: true
tendermint_mempool_recheck_empty: true
tendermint_mempool_broadcast: true
tendermint_mempool_wal_dir: "{{tendermint_db_dir}}/mempool.wal"

tendermint_log_file: /var/log/tendermint.log

tendermint_chain_id: mychain
tendermint_genesis_time: "{{ansible_date_time.iso8601_micro}}"
tendermint_validators: []
```

## Handlers

These are the handlers that are defined in `handlers/main.yml`.

* `restart tendermint`
* `reload systemd`

## Example playbook that configures a Tendermint on Ubuntu 14.04

```
---

- hosts: all
  vars:
    tendermint_chain_id: MyAwesomeChain
    tendermint_seeds: "172.13.0.1:46656,172.13.0.2:46656,172.13.0.3:46656"
  roles:
    - ansible-tendermint
```

This playbook will install Tendermint and will create all the
required directories. But **it won't start the Tendermint if no
validators were given**.

You will need to collect validators public keys manually or using
`collect_public_keys.yml` given you have SSH access to all the nodes and set `tendermint_validators` variable:

```
---

- hosts: all
  vars:
    tendermint_chain_id: MyAwesomeChain
    tendermint_seeds: "172.13.0.1:46656,172.13.0.2:46656,172.13.0.3:46656"
    tendermint_validators:
      - pub_key:
          - 1
          - 1F017E488A6327FAFBBE092193B427912E117733DE6AF72150BF09AA58411E7F
        amount: 10
        name: paris
  roles:
    - ansible-tendermint
```

### Example playbook that configures a Tendermint with in-proc dummy app

```
---

- hosts: all
  vars:
    tendermint_chain_id: MyAwesomeChain
    tendermint_proxy_app: dummy
  roles:
    - ansible-tendermint
```

## Testing

```
vagrant up
```
