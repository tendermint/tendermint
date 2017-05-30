// The cluster name, e.g cdn
output "name" {
  value = "${var.name}"
}

// The list of cluster instance ids
output "instances" {
  value = ["${digitalocean_droplet.cluster.*.id}"]
}

// The list of cluster instance ips
output "private_ips" {
  value = ["${digitalocean_droplet.cluster.*.ipv4_address_private}"]
}

// The list of cluster instance ips
output "public_ips" {
  value = ["${digitalocean_droplet.cluster.*.ipv4_address}"]
}
