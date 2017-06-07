resource "digitalocean_tag" "cluster" {
  name = "${var.name}"
}

resource "digitalocean_droplet" "cluster" {
  # set the image and instance type
  name = "${var.name}-node${count.index}"
  image = "${var.image_id}"
  size = "${var.instance_size}"
  # the `element` function handles modulo
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

#  #Additional SSH security: add ec2-user and remove root user credentials. You need to have ssh-agent running with your key for this to work.
#  provisioner "remote-exec" {
#    inline = [
#      "useradd -m -s /bin/bash ec2-user",
#      "echo 'ec2-user ALL=(ALL) NOPASSWD:ALL' > /etc/sudoers.d/ec2-user",
#      "cp -r /root/.ssh /home/ec2-user/.ssh",
#      "chown -R ec2-user.ec2-user /home/ec2-user/.ssh",
#      "chmod -R 700 /home/ec2-user/.ssh",
#      "rm -rf /root/.ssh"
#    ]
#  }

}

#resource "digitalocean_floating_ip" "cluster" {
#  droplet_id = "${element(digitalocean_droplet.cluster.*.id,count.index)}"
#  region     = "${element(digitalocean_droplet.cluster.*.region,count.index)}"
#  count      = "${var.servers}"
#}

