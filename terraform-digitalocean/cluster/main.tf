resource "digitalocean_tag" "cluster" {
  name = "${var.name}"
}

resource "digitalocean_droplet" "cluster" {
  name = "${var.name}-node${count.index}"
  image = "${var.image_id}"
  size = "${var.instance_size}"
  region = "${element(var.regions, count.index)}"
  ssh_keys = "${var.key_ids}"
  count = "${var.servers}"
  tags = ["${digitalocean_tag.cluster.id}"]

  lifecycle = {
	prevent_destroy = false
  }

  connection {
    timeout = "30s"
  }

}

