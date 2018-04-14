Using Ansible
=============

.. figure:: assets/a_plus_t.png
   :alt: Ansible plus Tendermint

   Ansible plus Tendermint

The playbooks in `our ansible directory <https://github.com/tendermint/tools/tree/master/ansible>`__ 
run ansible `roles <http://www.ansible.com/>`__ which:

-  install and configure basecoind or ethermint
-  start/stop basecoind or ethermint and reset their configuration

Prerequisites
-------------

-  Ansible 2.0 or higher
-  SSH key to the servers

Optional for DigitalOcean droplets:

- DigitalOcean API Token
- python dopy package

For a description on how to get a DigitalOcean API Token, see the explanation
in the `using terraform tutorial <./terraform-digitalocean.html>`__.

Optional for Amazon AWS instances: 

- Amazon AWS API access key ID and secret access key.

The cloud inventory scripts come from the ansible team at their
`GitHub <https://github.com/ansible/ansible>`__ page. You can get the
latest version from the ``contrib/inventory`` folder.

Setup
-----

Ansible requires a "command machine" or "local machine" or "orchestrator
machine" to run on. This can be your laptop or any machine that can run
ansible. (It does not have to be part of the cloud network that hosts
your servers.)

Use the official `Ansible installation
guide <http://docs.ansible.com/ansible/intro_installation.html>`__ to
install Ansible. Here are a few examples on basic installation commands:

Ubuntu/Debian:

::

    sudo apt-get install ansible

CentOS/RedHat:

::

    sudo yum install epel-release
    sudo yum install ansible

Mac OSX: If you have `Homebrew <https://brew.sh>`__ installed, then it's:

::

    brew install ansible

If not, you can install it using ``pip``:

::

    sudo easy_install pip
    sudo pip install ansible

To make life easier, you can start an SSH Agent and load your SSH
key(s). This way ansible will have an uninterrupted way of connecting to
your servers.

::

    ssh-agent > ~/.ssh/ssh.env
    source ~/.ssh/ssh.env

    ssh-add private.key

Subsequently, as long as the agent is running, you can use
``source ~/.ssh/ssh.env`` to load the keys to the current session. Note:
On Mac OSX, you can add the ``-K`` option to ssh-add to store the
passphrase in your keychain. The security of this feature is debated but
it is convenient.

Optional cloud dependencies
~~~~~~~~~~~~~~~~~~~~~~~~~~~

If you are using a cloud provider to host your servers, you need the
below dependencies installed on your local machine.

DigitalOcean inventory dependencies:
^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^

Ubuntu/Debian:

::

    sudo apt-get install python-pip
    sudo pip install dopy

CentOS/RedHat:

::

    sudo yum install python-pip
    sudo pip install dopy

Mac OSX:

::

    sudo pip install dopy

Amazon AWS inventory dependencies:
^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^

Ubuntu/Debian:

::

    sudo apt-get install python-boto

CentOS/RedHat:

::

    sudo yum install python-boto

Mac OSX:

::

    sudo pip install boto

Refreshing the DigitalOcean inventory
^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^

If you just finished creating droplets, the local DigitalOcean inventory
cache is not up-to-date. To refresh it, run:

::

    DO_API_TOKEN="<The API token received from DigitalOcean>"
    python -u inventory/digital_ocean.py --refresh-cache 1> /dev/null

Refreshing the Amazon AWS inventory
^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^

If you just finished creating Amazon AWS EC2 instances, the local AWS
inventory cache is not up-to-date. To refresh it, run:

::

    AWS_ACCESS_KEY_ID='<The API access key ID received from Amazon>'
    AWS_SECRET_ACCESS_KEY='<The API secret access key received from Amazon>'
    python -u inventory/ec2.py --refresh-cache 1> /dev/null

Note: you don't need the access key and secret key set, if you are
running ansible on an Amazon AMI instance with the proper IAM
permissions set.

Running the playbooks
---------------------

The playbooks are locked down to only run if the environment variable
``TF_VAR_TESTNET_NAME`` is populated. This is a precaution so you don't
accidentally run the playbook on all your servers.

The variable ``TF_VAR_TESTNET_NAME`` contains the testnet name which
ansible translates into an ansible group. If you used Terraform to
create the servers, it was the testnet name used there.

If the playbook cannot connect to the servers because of public key
denial, your SSH Agent is not set up properly. Alternatively you can add
the SSH key to ansible using the ``--private-key`` option.

If you need to connect to the nodes as root but your local username is
different, use the ansible option ``-u root`` to tell ansible to connect
to the servers and authenticate as the root user.

If you secured your server and you need to ``sudo`` for root access, use
the the ``-b`` or ``--become`` option to tell ansible to sudo to root
after connecting to the server. In the Terraform-DigitalOcean example,
if you created the ec2-user by adding the ``noroot=true`` option (or if
you are simply on Amazon AWS), you need to add the options
``-u ec2-user -b`` to ansible to tell it to connect as the ec2-user and
then sudo to root to run the playbook.

DigitalOcean
~~~~~~~~~~~~

::

    DO_API_TOKEN="<The API token received from DigitalOcean>"
    TF_VAR_TESTNET_NAME="testnet-servers"
    ansible-playbook -i inventory/digital_ocean.py install.yml -e service=basecoind

Amazon AWS
~~~~~~~~~~

::

    AWS_ACCESS_KEY_ID='<The API access key ID received from Amazon>'
    AWS_SECRET_ACCESS_KEY='<The API secret access key received from Amazon>'
    TF_VAR_TESTNET_NAME="testnet-servers"
    ansible-playbook -i inventory/ec2.py install.yml -e service=basecoind

Installing custom versions
~~~~~~~~~~~~~~~~~~~~~~~~~~

By default ansible installs the tendermint, basecoind or ethermint binary
versions from the latest release in the repository. If you build your
own version of the binaries, you can tell ansible to install that
instead.

::

    GOPATH="<your go path>"
    go get -u github.com/tendermint/basecoin/cmd/basecoind

    DO_API_TOKEN="<The API token received from DigitalOcean>"
    TF_VAR_TESTNET_NAME="testnet-servers"
    ansible-playbook -i inventory/digital_ocean.py install.yml -e service=basecoind -e release_install=false

Alternatively you can change the variable settings in
``group_vars/all``.

Other commands and roles
------------------------

There are few extra playbooks to make life easier managing your servers.

-  install.yml - Install basecoind or ethermint applications. (Tendermint
   gets installed automatically.) Use the ``service`` parameter to
   define which application to install. Defaults to ``basecoind``.
-  reset.yml - Stop the application, reset the configuration and data,
   then start the application again. You need to pass
   ``-e service=<servicename>``, like ``-e service=basecoind``. It will
   restart the underlying tendermint application too.
-  restart.yml - Restart a service on all nodes. You need to pass
   ``-e service=<servicename>``, like ``-e service=basecoind``. It will
   restart the underlying tendermint application too.
-  stop.yml - Stop the application. You need to pass
   ``-e service=<servicename>``.
-  status.yml - Check the service status and print it. You need to pass
   ``-e service=<servicename>``.
-  start.yml - Start the application. You need to pass
   ``-e service=<servicename>``.
-  ubuntu16-patch.yml - Ubuntu 16.04 does not have the minimum required
   python package installed to be able to run ansible. If you are using
   ubuntu, run this playbook first on the target machines. This will
   install the python pacakge that is required for ansible to work
   correctly on the remote nodes.
-  upgrade.yml - Upgrade the ``service`` on your testnet. It will stop
   the service and restart it at the end. It will only work if the
   upgraded version is backward compatible with the installed version.
-  upgrade-reset.yml - Upgrade the ``service`` on your testnet and reset
   the database. It will stop the service and restart it at the end. It
   will work for upgrades where the new version is not
   backward-compatible with the installed version - however it will
   reset the testnet to its default.

The roles are self-sufficient under the ``roles/`` folder.

-  install - install the application defined in the ``service``
   parameter. It can install release packages and update them with
   custom-compiled binaries.
-  unsafe\_reset - delete the database for a service, including the
   tendermint database.
-  config - configure the application defined in ``service``. It also
   configures the underlying tendermint service. Check
   ``group_vars/all`` for options.
-  stop - stop an application. Requires the ``service`` parameter set.
-  status - check the status of an application. Requires the ``service``
   parameter set.
-  start - start an application. Requires the ``service`` parameter set.

Default variables
-----------------

Default variables are documented under ``group_vars/all``. You can the
parameters there to deploy a previously created genesis.json file
(instead of dynamically creating it) or if you want to deploy custom
built binaries instead of deploying a released version.
