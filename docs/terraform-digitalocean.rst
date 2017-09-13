Terraform for Digital Ocean
===========================

This is a generic `Terraform <https://www.terraform.io/>`__
configuration that sets up DigitalOcean droplets.

Prerequisites
=============

-  Install `HashiCorp Terraform <https://www.terraform.io>`__ on a linux
   machine.
-  Create a `DigitalOcean API
   token <https://cloud.digitalocean.com/settings/api/tokens>`__ with
   read and write capability.
-  Create a private/public key pair for SSH. This is needed to log onto
   your droplets as well as by Ansible to connect for configuration
   changes.
-  Set up the public SSH key at the `DigitalOcean security
   page <https://cloud.digitalocean.com/settings/security>`__.
   `Here <https://www.digitalocean.com/community/tutorials/how-to-use-ssh-keys-with-digitalocean-droplets>`__'s
   a tutorial.
-  Find out your SSH key ID at DigitalOcean by querying the below
   command on your linux box:

::

    DO_API_TOKEN="<The API token received from DigitalOcean>"
    curl -X GET -H "Content-Type: application/json" -H "Authorization: Bearer $DO_API_TOKEN" "https://api.digitalocean.com/v2/account/keys"

How to run
==========

Initialization
--------------

If this is your first time using terraform, you have to initialize it by
running the below command. (Note: initialization can be run multiple
times)

::

    terraform init

After initialization it's good measure to create a new Terraform
environment for the droplets so they are always managed together.

::

    TESTNET_NAME="testnet-servers"
    terraform env new "$TESTNET_NAME"

Note this ``terraform env`` command is only available in terraform
``v0.9`` and up.

Execution
---------

The below command will create 4 nodes in DigitalOcean. They will be
named ``testnet-servers-node0`` to ``testnet-servers-node3`` and they
will be tagged as ``testnet-servers``.

::

    DO_API_TOKEN="<The API token received from DigitalOcean>"
    SSH_IDS="[ \"<The SSH ID received from the curl call above.>\" ]"
    terraform apply -var TESTNET_NAME="testnet-servers" -var servers=4 -var DO_API_TOKEN="$DO_API_TOKEN" -var ssh_keys="$SSH_IDS"

Note: ``ssh_keys`` is a list of strings. You can add multiple keys. For
example: ``["1234567","9876543"]``.

Alternatively you can use the default settings. The number of default
servers is 4 and the testnet name is ``tf-testnet1``. Variables can also
be defined as environment variables instead of the command-line.
Environment variables that start with ``TF_VAR_`` will be translated
into the Terraform configuration. For example the number of servers can
be overriden by setting the ``TF_VAR_servers`` variable.

::

    TF_VAR_DO_API_TOKEN="<The API token received from DigitalOcean>"
    TF_VAR_TESTNET_NAME="testnet-servers"
    terraform-apply

Security
--------

DigitalOcean uses the root user by default on its droplets. This is fine
as long as SSH keys are used. However some people still would like to
disable root and use an alternative user to connect to the droplets -
then ``sudo`` from there. Terraform can do this but it requires SSH
agent running on the machine where terraform is run, with one of the SSH
keys of the droplets added to the agent. (This will be neede for ansible
too, so it's worth setting it up here. Check out the
`ansible <https://github.com/tendermint/tools/tree/master/ansible>`__
page for more information.) After setting up the SSH key, run
``terraform apply`` with ``-var noroot=true`` to create your droplets.
Terraform will create a user called ``ec2-user`` and move the SSH keys
over, this way disabling SSH login for root. It also adds the
``ec2-user`` to the sudoers file, so after logging in as ec2-user you
can ``sudo`` to ``root``.

DigitalOcean announced firewalls but the current version of Terraform
(0.9.8 as of this writing) does not support it yet. Fortunately it is
quite easy to set it up through the web interface (and not that bad
through the `RESTful
API <https://developers.digitalocean.com/documentation/v2/#firewalls>`__
either). When adding droplets to a firewall rule, you can add tags. All
droplets in a testnet are tagged with the testnet name so it's enough to
define the testnet name in the firewall rule. It is not necessary to add
the nodes one-by-one. Also, the firewall rule "remembers" the testnet
name tag so if you change the servers but keep the name, the firewall
rules will still apply.

What's next
===========

After setting up the nodes, head over to the `ansible
folder <https://github.com/tendermint/tools/tree/master/ansible>`__ to
set up tendermint and basecoin.
