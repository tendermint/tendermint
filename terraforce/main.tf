module "cluster" {
  source      = "./cluster"
  environment = "test"
  name        = "tendermint-testnet"

  # curl -X GET -H "Content-Type: application/json" -H "Authorization: Bearer $DIGITALOCEAN_TOKEN" "https://api.digitalocean.com/v2/account/keys"
  key_ids = [8163311]

  image_id = "ubuntu-14-04-x64"
  desired_capacity = 4
  instance_size = "2gb"

  regions = ["AMS2", "FRA1", "LON1", "NYC2", "SFO2", "SGP1", "TOR1"]
}


provider "digitalocean" {
}

output "public_ips" {
  value = "${module.cluster.public_ips}"
}

output "private_ips" {
  value = "${join(",",module.cluster.private_ips)}"
}

output "seeds" {
  value = "${join(":46656,",module.cluster.public_ips)}:46656"
}
