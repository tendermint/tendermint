variable "name" {
  description = "The cluster name, e.g cdn"
}

variable "image_id" {
  description = "Image ID"
  default = "ubuntu-16-04-x64"
}

variable "regions" {
  description = "Regions to launch in"
  type = "list"
  default = ["AMS2", "FRA1", "LON1", "NYC3", "SFO2", "SGP1", "TOR1"]
}

variable "key_ids" {
  description = "SSH keys to use on the nodes"
  type = "list"
}

variable "instance_size" {
  description = "The instance size to use"
  default = "2gb"
}

variable "servers" {
  description = "Desired instance count"
  default     = 4
}

variable "noroot" {
  description = "Set this variable to true, if you want SSH keys set for ec2-user instead of root."
  default     = false
}

