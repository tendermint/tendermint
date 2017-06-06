# Terraform for Digital Ocean

This is a generic [Terraform](https://www.terraform.io/) configuration that sets up DigitalOcean droplets.

# Prerequisites

* Install [HashiCorp Terraform](https://www.terraform.io) on a linux machine.
* Create a [DigitalOcean API token](https://cloud.digitalocean.com/settings/api/tokens) with read and write capability.
* Set an SSH key at the [DigitalOcean security page](https://cloud.digitalocean.com/settings/security). {Here](https://www.digitalocean.com/community/tutorials/how-to-use-ssh-keys-with-digitalocean-droplets)'s a tutorial.
* Find out your SSH key ID at DigitalOcean by querying the below command on your linux box:

```
DO_API_TOKEN="<The API token received from DigitalOcean>"
curl -X GET -H "Content-Type: application/json" -H "Authorization: Bearer $DO_API_TOKEN" "https://api.digitalocean.com/v2/account/keys"
```

# How to run

## Initialization
If this is your first time using terraform, you have to initialize it by running the below command. (Note: initialization can be run multiple times)
```
terraform init
```

After initialization it's good measure to create a new Terraform environment for the droplets so they are always managed together.

```
TESTNET_NAME="testnet-servers"
terraform env new "$TESTNET_NAME"
```

Note this `terraform env` command is only available in terraform `v0.9` and up.

## Execution

The below command will create 4 nodes in DigitalOcean. They will be named `testnet-servers-node0` to `testnet-servers-node3` and they will be tagged as `testnet-servers`.
```
DO_API_TOKEN="<The API token received from DigitalOcean>"
SSH_IDS="[ \"<The SSH ID received from the curl call above.>\" ]"
terraform apply -var TESTNET_NAME="testnet-servers" -var servers=4 -var DO_API_TOKEN="$DO_API_TOKEN" -var ssh_keys="$SSH_IDS"
```

Note: `ssh_keys` is a list of strings. You can add multiple keys. For example: `["1234567","9876543"]`.

Alternatively you can use the default settings. The number of default servers is 4 and the testnet name is `tf-testnet1`. Variables can also be defined as environment variables instead of the command-line. Environment variables that start with `TF_VAR_` will be translated into the Terraform configuration. For example the number of servers can be overriden by setting the `TF_VAR_servers` variable.

```
TF_VAR_DO_API_TOKEN="<The API token received from DigitalOcean>"
TF_VAR_TESTNET_NAME="testnet-servers"
terraform-apply
```

# What's next

After setting up the nodes, head over to the [ansible folder](https://github.com/tendermint/tools) to set up tendermint and basecoin.


