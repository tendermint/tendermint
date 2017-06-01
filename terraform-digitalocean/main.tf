#Terraform Configuration

variable "DO_API_TOKEN" {
  description = "DigitalOcean Access Token"
}

variable "TESTNET_NAME" {
  description = "Name of the cluster/testnet"
  default = "tf-testnet1"
}

variable "ssh_keys" {
  description = "SSH keys provided in DigitalOcean to be used on the nodes"
  # curl -X GET -H "Content-Type: application/json" -H "Authorization: Bearer $DIGITALOCEAN_TOKEN" "https://api.digitalocean.com/v2/account/keys"
  default = ["9495227"]
}

variable "servers" {
  description = "Number of nodes in cluster"
  default = "4"
}

provider "digitalocean" {
  token = "${var.DO_API_TOKEN}"
}


module "cluster" {
  source           = "./cluster"
  name             = "${var.TESTNET_NAME}"
  key_ids          = "${var.ssh_keys}"
  servers          = "${var.servers}"
}


output "public_ips" {
  value = "${module.cluster.public_ips}"
}

#output "floating_ips" {
#  value = "${module.cluster.floating_ips}"
#}

