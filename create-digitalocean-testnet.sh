#!/bin/bash
# This is an example set of commands that uses Terraform and Ansible to create a testnet on Digital Ocean.

# Prerequisites: terraform, ansible, DigitalOcean API token, ssh-agent running with the same SSH keys added that are set up during terraform

#export DO_API_TOKEN="<This contains the DigitalOcean API token>"

TF_VAR_TESTNET_NAME="$1"

if [ -z "$TF_VAR_TESTNET_NAME" ]; then
  echo "Usage: $0 <TF_VAR_TESTNET_NAME>"
  echo "or"
  echo "export TF_VAR_TESTNET_NAME=<TF_VAR_TESTNET_NAME> ; $0"
  exit
fi

cd terraforce
terraform init
terraform env new "$TF_VAR_TESTNET_NAME"
terraform apply -var servers=4 -var DO_API_TOKEN="$DO_API_TOKEN"
cd ..

#Note that SSH Agent needs to be running with SSH keys added or ansible-playbook requires the --private-key option.
cd ansible
python -u inventory/digital_ocean.py --refresh-cache 1> /dev/null
ansible-playbook -i inventory/digital_ocean.py install.yml
cd ..

