Using Terraform
===============

This is a `Terraform <https://www.terraform.io/>`__ configuration that sets up DigitalOcean droplets.

Prerequisites
-------------

-  Install `HashiCorp Terraform <https://www.terraform.io>`__ on a linux machine.
-  Create a `DigitalOcean API token <https://cloud.digitalocean.com/settings/api/tokens>`__ with read and write capability.
-  Create SSH keys

Build
-----

::

    export DO_API_TOKEN="abcdef01234567890abcdef01234567890"
    export SSH_KEY_FILE="$HOME/.ssh/id_rsa.pub"

    terraform init
    terraform apply -var DO_API_TOKEN="$DO_API_TOKEN" -var SSH_KEY_FILE="$SSH_KEY_FILE"

At the end you will get a list of IP addresses that belongs to your new droplets.

Destroy
-------

Run the below:

::

    terraform destroy
