# Terraform & Ansible

Automated deployments are done using
[Terraform](https://www.terraform.io/) to create servers on Digital
Ocean then [Ansible](http://www.ansible.com/) to create and manage
testnets on those servers.

## Install

NOTE: see the [integration bash
script](https://github.com/tendermint/tendermint/blob/develop/networks/remote/integration.sh)
that can be run on a fresh DO droplet and will automatically spin up a 4
node testnet. The script more or less does everything described below.

- Install [Terraform](https://www.terraform.io/downloads.html) and
  [Ansible](http://docs.ansible.com/ansible/latest/installation_guide/intro_installation.html)
  on a Linux machine.
- Create a [DigitalOcean API
  token](https://cloud.digitalocean.com/settings/api/tokens) with read
  and write capability.
- Install the python dopy package (`pip install dopy`)
- Create SSH keys (`ssh-keygen`)
- Set environment variables:

```
export DO_API_TOKEN="abcdef01234567890abcdef01234567890"
export SSH_KEY_FILE="$HOME/.ssh/id_rsa.pub"
```

These will be used by both `terraform` and `ansible`.

## Terraform

This step will create four Digital Ocean droplets. First, go to the
correct directory:

```
cd $GOPATH/src/github.com/tendermint/tendermint/networks/remote/terraform
```

then:

```
terraform init
terraform apply -var DO_API_TOKEN="$DO_API_TOKEN" -var SSH_KEY_FILE="$SSH_KEY_FILE"
```

and you will get a list of IP addresses that belong to your droplets.

With the droplets created and running, let's setup Ansible.

## Ansible

The playbooks in [the ansible
directory](https://github.com/tendermint/tendermint/tree/master/networks/remote/ansible)
run ansible roles to configure the sentry node architecture. You must
switch to this directory to run ansible
(`cd $GOPATH/src/github.com/tendermint/tendermint/networks/remote/ansible`).

There are several roles that are self-explanatory:

First, we configure our droplets by specifying the paths for tendermint
(`BINARY`) and the node files (`CONFIGDIR`). The latter expects any
number of directories named `node0, node1, ...` and so on (equal to the
number of droplets created). For this example, we use pre-created files
from [this
directory](https://github.com/tendermint/tendermint/tree/master/docs/examples).
To create your own files, use either the `tendermint testnet` command or
review [manual deployments](./deploy-testnets.md).

Here's the command to run:

```
ansible-playbook -i inventory/digital_ocean.py -l sentrynet config.yml -e BINARY=$GOPATH/src/github.com/tendermint/tendermint/build/tendermint -e CONFIGDIR=$GOPATH/src/github.com/tendermint/tendermint/docs/examples
```

Voila! All your droplets now have the `tendermint` binary and required
configuration files to run a testnet.

Next, we run the install role:

```
ansible-playbook -i inventory/digital_ocean.py -l sentrynet install.yml
```

which as you'll see below, executes
`tendermint node --proxy_app=kvstore` on all droplets. Although we'll
soon be modifying this role and running it again, this first execution
allows us to get each `node_info.id` that corresponds to each
`node_info.listen_addr`. (This part will be automated in the future). In
your browser (or using `curl`), for every droplet, go to IP:26657/status
and note the two just mentioned `node_info` fields. Notice that blocks
aren't being created (`latest_block_height` should be zero and not
increasing).

Next, open `roles/install/templates/systemd.service.j2` and look for the
line `ExecStart` which should look something like:

```
ExecStart=/usr/bin/tendermint node --proxy_app=kvstore
```

and add the `--p2p.persistent_peers` flag with the relevant information
for each node. The resulting file should look something like:

```
[Unit]
Description={{service}}
Requires=network-online.target
After=network-online.target

[Service]
Restart=on-failure
User={{service}}
Group={{service}}
PermissionsStartOnly=true
ExecStart=/usr/bin/tendermint node --proxy_app=kvstore --p2p.persistent_peers=167b80242c300bf0ccfb3ced3dec60dc2a81776e@165.227.41.206:26656,3c7a5920811550c04bf7a0b2f1e02ab52317b5e6@165.227.43.146:26656,303a1a4312c30525c99ba66522dd81cca56a361a@159.89.115.32:26656,b686c2a7f4b1b46dca96af3a0f31a6a7beae0be4@159.89.119.125:26656
ExecReload=/bin/kill -HUP $MAINPID
KillSignal=SIGTERM

[Install]
WantedBy=multi-user.target
```

Then, stop the nodes:

```
ansible-playbook -i inventory/digital_ocean.py -l sentrynet stop.yml
```

Finally, we run the install role again:

```
ansible-playbook -i inventory/digital_ocean.py -l sentrynet install.yml
```

to re-run `tendermint node` with the new flag, on all droplets. The
`latest_block_hash` should now be changing and `latest_block_height`
increasing. Your testnet is now up and running :)

Peek at the logs with the status role:

```
ansible-playbook -i inventory/digital_ocean.py -l sentrynet status.yml
```

## Logging

The crudest way is the status role described above. You can also ship
logs to Logz.io, an Elastic stack (Elastic search, Logstash and Kibana)
service provider. You can set up your nodes to log there automatically.
Create an account and get your API key from the notes on [this
page](https://app.logz.io/#/dashboard/data-sources/Filebeat), then:

```
yum install systemd-devel || echo "This will only work on RHEL-based systems."
apt-get install libsystemd-dev || echo "This will only work on Debian-based systems."

go get github.com/mheese/journalbeat
ansible-playbook -i inventory/digital_ocean.py -l sentrynet logzio.yml -e LOGZIO_TOKEN=ABCDEFGHIJKLMNOPQRSTUVWXYZ012345
```

## Cleanup

To remove your droplets, run:

```
terraform destroy -var DO_API_TOKEN="$DO_API_TOKEN" -var SSH_KEY_FILE="$SSH_KEY_FILE"
```
