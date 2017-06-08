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
  # curl -X GET -H "Content-Type: application/json" -H "Authorization: Bearer $DO_API_TOKEN" "https://api.digitalocean.com/v2/account/keys"
  default = [
             "6259615",
             "7658963",
             "7668263",
             "7668264",
             "8036767",
             "8163311",
             "9495227"
            ]
}

variable "servers" {
  description = "Number of nodes in cluster"
  default = "4"
}

variable "noroot" {
  description = "Set this variable to true, if you want SSH keys set for ec2-user instead of root."
  default     = false
}

provider "digitalocean" {
  token = "${var.DO_API_TOKEN}"
}

module "cluster" {
  source           = "./cluster"
  name             = "${var.TESTNET_NAME}"
  key_ids          = "${var.ssh_keys}"
  servers          = "${var.servers}"
  noroot           = "${var.noroot}"
}


output "public_ips" {
  value = "${module.cluster.public_ips}"
}

#output "floating_ips" {
#  value = "${module.cluster.floating_ips}"
#}

