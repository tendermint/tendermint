## How to run ansible ##
Before running ansible you should do a manual step:
Add given IPs from your cluster to ansible_inventory and make sure that each of them has unique `mach_folder` variable corresponding to a proper folder.
After that run the following command:
`ansible-playbook ansible_tendermint_init.yml`


## This is temporal solution ##
To make Ansible working you need to create a record in */etc/hosts* file for 4 servers using real IP addresses.
It would look like that:
```
1.2.3.4 tendermint-node1
1.2.3.5 tendermint-node2
1.2.3.6 tendermint-node3
1.2.3.7 tendermint-node4
```

Later, we would generate a dynamic Ansible inventory from a Terraform state file. [https://github.com/adammck/terraform-inventory]

### Key file ###
You should have a private key on your laptop at `~/.ssh/tendermint_bot_key` path and a corresponding public key on remote server.
This is controlled by `ansible/ansible.cfg` file at line 77:
`private_key_file = ~/.ssh/tendermint_bot_key`

As a workaround for that we could keep private key value inside encrypted var file at `group_vars` folder

## Source bins and run.sh files to Ansible ##
We could integrate Ansible as a part of `[testnet|https://github.com/tendermint/tendermint/tree/testnet/test/net]` and symlink roles/tendermint_init/files to `[CONFIG|https://github.com/tendermint/tendermint/blob/testnet/test/net/scripts/init.sh#L6]` folder containing `bins` and `run.sh` files.
