# Ansible playbook for Tendermint

![Ansible plus Tendermint](img/a_plus_t.png)

* [Prerequisites](#Prerequisites)
* [Ansible setup](#Ansible setup)
* [Running the playbook](#Running the playbook)
* [Example playbook that configures a Tendermint on Ubuntu](#example-playbook-that-configures-a-tendermint-on-ubuntu)

The playbooks in this folder run [ansible](http://www.ansible.com/) roles which:

* install tendermint
* install basecoin
* configure tendermint and basecoin
* start/stop tendermint and basecoin and reset their configuration

## Prerequisites

* Ansible 2.0 or higher
* SSH key to the servers

Optional for DigitalOcean droplets:
* DigitalOcean API Token
* python dopy package

Head over to the [Terraform folder](https://github.com/tendermint/tools) for a description on how to get a DigitalOcean API Token.

Optional for Amazon AWS instances:
* Amazon AWS API access key ID and secret access key.

The cloud inventory scripts come from the ansible team at their [GitHub](https://github.com/ansible/ansible) page. You can get the latest version from the contrib/inventory folder.

## Ansible setup

Ansible requires a "command machine" or "local machine" or "orchestrator machine" to run on. This can be your laptop or any machine that runs linux. (It does not have to be part of the cloud network that hosts your servers.)

Use the official [Ansible installation guide](http://docs.ansible.com/ansible/intro_installation.html) to install Ansible. Here are a few examples on basic installation commands:

Ubuntu/Debian:
```
sudo apt-get install ansible
```

CentOS/RedHat:
```
sudo yum install epel-release
sudo yum install ansible
```

Mac OSX:
```
sudo easy_install pip
sudo pip install ansible
```

To make life easier, you can start an SSH Agent and load your SSH key(s). This way ansible will have an uninterrupted way of connecting to your servers.

```
ssh-agent > ~/.ssh/ssh.env
source ~/.ssh/ssh.env

ssh-add private.key
```

Subsequently, as long as the agent is running, you can use `source ~/.ssh/ssh.env` to load the keys to the current session.

### Optional cloud dependencies

If you are using a cloud provider to host your servers, you need the below dependencies installed on your local machine.

#### DigitalOcean inventory dependencies:

Ubuntu/Debian:
```
sudo apt-get install python-pip
sudo pip install dopy
```

CentOS/RedHat:
```
sudo yum install python-pip
sudo pip install dopy
```

Mac OSX:
```
sudo pip install dopy
```

#### Amazon AWS inventory dependencies:

Ubuntu/Debian:
```
sudo apt-get install python-boto
```

CentOS/RedHat:
```
sudo yum install python-boto
```

Mac OSX:
```
sudo pip install boto
```

## Refreshing the DigitalOcean inventory

If you just finished creating droplets, the local DigitalOcean inventory cache is not up-to-date. To refresh it, run:

```
DO_API_TOKEN="<The API token received from DigitalOcean>"
python -u inventory/digital_ocean.py --refresh-cache 1> /dev/null
```

## Refreshing the Amazon AWS inventory

If you just finished creating Amazon AWS EC2 instances, the local AWS inventory cache is not up-to-date. To refresh it, run:

```
AWS_ACCESS_KEY_ID='<The API access key ID received from Amazon>'
AWS_SECRET_ACCESS_KEY='<The API secret access key received from Amazon>'
python -u inventory/ec2.py --refresh-cache 1> /dev/null
```

Note: you don't need the access key and secret key set, if you are running ansible on an Amazon AMI instance with the proper IAM permissions set.

## Running the playbook

The playbook is locked down to only run if the environment variable `TF_VAR_TESTNET_NAME` is populated. This is a precaution so you don't accidentally run the playbook on all your servers.

The variable `TF_VAR_TESTNET_NAME` contains the testnet name which ansible translates into an ansible group. If you used Terraform to create the servers, it was the testnet name used there.

If the playbook cannot connect to the servers because of public key denial, your SSH Agent is not set up properly. Alternatively you can add the SSH key to ansible using the `--private-key` option.

### DigitalOcean
```
DO_API_TOKEN="<The API token received from DigitalOcean>"
TF_VAR_TESTNET_NAME="testnet-servers"
ansible-playbook -i inventory/digital_ocean.py install.yml
```

### Amazon AWS
```
AWS_ACCESS_KEY_ID='<The API access key ID received from Amazon>'
AWS_SECRET_ACCESS_KEY='<The API secret access key received from Amazon>'
TF_VAR_TESTNET_NAME="testnet-servers"
ansible-playbook -i inventory/ec2.py install.yml
```

### Installing custom versions

By default ansible installs the tendermint and basecoin binary versions defined in its [default variables](#Default variables). If you build your own version of the binaries, you can tell ansible to install that instead.

```
GOPATH="<your go path>"
go get -u github.com/tendermint/tendermint/cmd/tendermint
go get -u github.com/tendermint/basecoin/cmd/basecoin

DO_API_TOKEN="<The API token received from DigitalOcean>"
TF_VAR_TESTNET_NAME="testnet-servers"
ansible-playbook -i inventory/digital_ocean.py install.yml -e tendermint_release_install=false -e basecoin_release_install=false
```

Alternatively you can change the variable settings in `group_vars/all`.

## Other commands and roles

There are few extra playbooks to make life easier managing your servers.

* install.yml - the all-in-one playbook to install and configure tendermint + basecoin
* reset.yml - stop the application, reset the configuration (blockchain), then start the application again
* stop.yml - stop the application
* start.yml - start the application
* restart.yml - restart the application

The roles are self-sufficient under the `roles/` folder.

* install-tendermint - install the tendermint application. It can install release packages or custom-compiled binaries.
* install-basecoin - install the basecoin application. It can install release packages or custom-compiled binaries.
* cleanupconfig - delete all tendermint and basecoin configuration.
* config - configure tendermint and basecoin
* stop - stop the application.
* start - start the application.

## Default variables

Default variables are documented under `group_vars/all`.

