/**
 * Cluster on DO
 *
 */

variable "name" {
  description = "The cluster name, e.g cdn"
}

variable "environment" {
  description = "Environment tag, e.g prod"
}

variable "image_id" {
  description = "Image ID"
}

variable "regions" {
  description = "Regions to launch in"
  type = "list"
}

variable "key_ids" {
  description = "SSH keys to use"
  type = "list"
}

variable "instance_size" {
  description = "The instance size to use, e.g 2gb"
}

variable "desired_capacity" {
  description = "Desired instance count"
  default     = 3
}

#-----------------------
# Instances

resource "digitalocean_droplet" "cluster" {
  # set the image and instance type
  name = "${var.name}${count.index}"
  image = "${var.image_id}"
  size = "${var.instance_size}"

  # the `element` function handles modulo
  region = "${element(var.regions, count.index)}"

  ssh_keys = "${var.key_ids}"

  count = "${var.desired_capacity}"
  lifecycle = {
	prevent_destroy = false
  }
}

#-----------------------

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
