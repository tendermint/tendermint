// The cluster name
output "name" {
  value = "${var.name}"
}

// The list of cluster instance IDs
output "instances" {
  value = ["${digitalocean_droplet.cluster.*.id}"]
}

// The list of cluster instance private IPs
output "private_ips" {
  value = ["${digitalocean_droplet.cluster.*.ipv4_address_private}"]
}

// The list of cluster instance public IPs
output "public_ips" {
  value = ["${digitalocean_droplet.cluster.*.ipv4_address}"]
}

#// The list of cluster floating IPs
#output "floating_ips" {
#  value = ["${digitalocean_floating_ip.cluster.*.ip_address}"]
#}

