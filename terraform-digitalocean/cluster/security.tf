resource "null_resource" "cluster" {
  count = "${ var.noroot ? var.servers : 0 }"
  connection {
    host = "${element(digitalocean_droplet.cluster.*.ipv4_address,count.index)}"
  }
  provisioner "remote-exec" {
    inline = [
      "useradd -m -s /bin/bash ec2-user",
      "echo 'ec2-user ALL=(ALL) NOPASSWD:ALL' > /etc/sudoers.d/ec2-user",
      "cp -r /root/.ssh /home/ec2-user/.ssh",
      "chown -R ec2-user.ec2-user /home/ec2-user/.ssh",
      "chmod -R 700 /home/ec2-user/.ssh",
      "rm -rf /root/.ssh"
    ]
  }
}

