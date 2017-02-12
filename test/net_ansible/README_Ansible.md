## How to run ansible ##
Before running ansible you should do a manual step:
Add given IPs from your cluster to ansible_inventory and make sure that each of them has unique `mach_folder` variable corresponding to a proper folder.
After that run the following command:
`ansible-playbook ansible_tendermint_init.yml`


#### This is temporal solution ####
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
Ansible.cfg points to a default private SSH key name at default location in most distributives: `~/.ssh/id_rsa`.
We assume that you use that key to manage logins on remote instances provisioned to run tendermint.
If you want to iuse another key, you could change it at `ansible/ansible.cfg` file at line 77:
`private_key_file = ~/.ssh/<ANOTHER_KEY>`


## Local VS remote binary ##
By default we download binary from S3 location and check SHA256 checksum for it.
However if you want to upload binary compiled locally, you could simply uncomment the following line in `vars` section: `local_binary = true`


### How to make Ansible working while on non-master branch ###
As tendermint is written in Go which doesn't allow usage of branches upon `go get` command, we should run ansible from a location which has necessary branch and symlink missing folders 
