# Ansible playbook for Tendermint on DigitalOcean

![Ansible plus Tendermint](img/a_plus_t.png)

* [Prerequisites](#Prerequisites)
* [Ansible setup](#Ansible setup)
* [Running the playbook](#Running the playbook)
* [Example playbook that configures a Tendermint on Ubuntu](#example-playbook-that-configures-a-tendermint-on-ubuntu)

The playbook in this folder contains [ansible](http://www.ansible.com/) roles which:

* installs tendermint
* configures tendermint
* configures tendermint service
* installs basecoin
* configures basecoin

## Prerequisites

* Ansible 2.0 or higher
* DigitalOcean API Token
* SSH key to the servers
* python dopy package

Head over to the [Terraform folder](https://github.com/tendermint/tools) for a description on how to get a DigitalOcean API Token.

The DigitalOcean inventory script comes from the ansible team at https://github.com/ansible/ansible. You can get the latest version from the contrib/inventory folder.

## Ansible setup

Ansible requires a "command machine" or "local machine" or "orchestrator machine" to run on. This can be your laptop or any machine that runs linux. (It does not have to be part of the DigitalOcean network.)

Example on RedHat/CentOS:
```
sudo yum install ansible python-pip
sudo pip install dopy
```

Example on Ubuntu/Debian:
```
sudo apt-get install ansible python-pip
sudo pip install dopy
```

To make life easier, you can start an SSH Agent and load your SSH key(s) into it. This way ansible will have an uninterrupted way of connecting to the droplets.

```
ssh-agent > ~/.ssh/ssh.env
source ~/.ssh/ssh.env

ssh-add private.key
```

Subsequently, as long as the agent is running, you can use `source ~/.ssh/ssh.env` to load the keys to the current session.

## Refreshing the DigitalOcean inventory

If you just finished creating droplets, the local DigitalOcean inventory cache is not up-to-date. To refresh it, run:

```
DO_API_TOKEN="<The API token received from DigitalOcean>"
python -u inventory/digital_ocean.py --refresh-cache 1> /dev/null
```

## Running the playbook

The playbook is locked down to only run if the environment variable `TF_VAR_TESTNET_NAME` is populated. This is a precaution so you don't accidentally run the playbook on all your DigitalOcean droplets.

The variable `TF_VAR_TESTNET_NAME` contains the testnet name defined when the droplets were created using Terraform.

```
TF_VAR_TESTNET_NAME="testnet-servers"
ansible-playbook -i inventory/digital_ocean.py install.yml
```

If the playbook cannot connect to the servers because of public key denial, your SSH Agent is not set up properly. Alternatively you can add the SSH key to ansible using the `--private-key` option.

## Starting the cluster

To be continued...

## Role details

To be continued...
