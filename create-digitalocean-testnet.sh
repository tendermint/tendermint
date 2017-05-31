#!/bin/bash
# This is an example set of commands that uses Terraform and Ansible to create a testnet on Digital Ocean.

# Prerequisites: terraform, ansible, DigitalOcean API token, ssh-agent running with the same SSH keys added that are set up during terraform
# Optional: GOPATH if you build the app yourself
#export DO_API_TOKEN="<This contains the DigitalOcean API token>"
#export GOPATH="<your go path>"

###
# Find out TF_VAR_TESTNET_NAME (testnet name)
###
if [ $# -gt 0 ]; then
  TF_VAR_TESTNET_NAME="$1"
fi

if [ -z "$TF_VAR_TESTNET_NAME" ]; then
  echo "Usage: $0 <TF_VAR_TESTNET_NAME>"
  echo "or"
  echo "export TF_VAR_TESTNET_NAME=<TF_VAR_TESTNET_NAME> ; $0"
  exit
fi

###
# Build Digital Ocean infrastructure
###
SERVERS=2
cd terraform-digitalocean
terraform init
terraform env new "$TF_VAR_TESTNET_NAME"
terraform apply -var servers=$SERVERS -var DO_API_TOKEN="$DO_API_TOKEN"
cd ..

###
# Build applications (optional)
###
if [ -n "$GOPATH" ]; then
  go get -u github.com/tendermint/tendermint/cmd/tendermint
  go get -u github.com/tendermint/basecoin/cmd/basecoin
  ANSIBLE_ADDITIONAL_VARS="-e tendermint_release_install=false -e basecoin_release_intall=false"
fi

###
# Deploy application
###
#Note that SSH Agent needs to be running with SSH keys added or ansible-playbook requires the --private-key option.
cd ansible
python -u inventory/digital_ocean.py --refresh-cache 1> /dev/null
ansible-playbook -i inventory/digital_ocean.py install.yml $ANSIBLE_ADDITIONAL_VARS
cd ..

###
# Start application
###
cd ansible
ansible-playbook -i inventory/digital_ocean.py start.yml
cd ..

