#resource "digitalocean_floating_ip" "cluster" {
#  droplet_id = "${element(digitalocean_droplet.cluster.*.id,count.index)}"
#  region     = "${element(digitalocean_droplet.cluster.*.region,count.index)}"
#  count      = "${var.servers}"
#}

provider "aws" {
}

data "aws_route53_zone" "cluster" {
  name         = "testnets.interblock.io."
}

resource "aws_route53_record" "cluster-node" {
  zone_id = "${data.aws_route53_zone.cluster.zone_id}"
  name    = "${var.name}-node${count.index}"
  type    = "A"
  ttl     = "300"
  records = ["${element(digitalocean_droplet.cluster.*.ipv4_address,count.index)}"]
  count      = "${var.servers}"
}

resource "aws_route53_record" "cluster-regions" {
  zone_id = "${data.aws_route53_zone.cluster.zone_id}"
  name    = "${var.name}-${element(digitalocean_droplet.cluster.*.region,count.index)}"
  type    = "CNAME"
  ttl     = "300"
  records = ["${element(aws_route53_record.cluster-node.*.name,count.index)}.${data.aws_route53_zone.cluster.name}"]
  count      = "${var.servers}"
}

