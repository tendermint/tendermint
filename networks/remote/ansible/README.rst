Using Ansible
=============

.. figure:: assets/a_plus_t.png
   :alt: Ansible plus Tendermint

   Ansible plus Tendermint

The playbooks in `the ansible directory <https://github.com/tendermint/tendermint/tree/master/networks/remote/ansible>`__ 
run ansible `roles <http://www.ansible.com/>`__ to configure the sentry node architecture.

Prerequisites
-------------

-  Install `Ansible 2.0 or higher <https://www.ansible.com>`__ on a linux machine.
-  Create a `DigitalOcean API token <https://cloud.digitalocean.com/settings/api/tokens>`__ with read and write capability.
-  Create SSH keys
-  Install the python dopy package (for the digital_ocean.py script)

Build
-----

::

    export DO_API_TOKEN="abcdef01234567890abcdef01234567890"
    export SSH_KEY_FILE="$HOME/.ssh/id_rsa.pub"

    
    ansible-playbook -i inventory/digital_ocean.py -l sentrynet install.yml

    # The scripts assume that you have your validator set up already.
    # You can create the folder structure for the sentry nodes using `tendermint testnet`.
    # For example: tendermint testnet --v 0 --n 4 --o build/
    # Then copy your genesis.json and modify the config.toml as you see fit.

    # Reconfig the sentry nodes with a new BINARY and the configuration files from the build folder:
    ansible-playbook -i inventory/digital_ocean.py -l sentrynet config.yml -e BINARY=`pwd`/build/tendermint -e CONFIGDIR=`pwd`/build

Shipping logs to logz.io
------------------------

Logz.io is an Elastic stack (Elastic search, Logstash and Kibana) service provider. You can set up your nodes to log there automatically. Create an account and get your API key from the notes on `this page <https://app.logz.io/#/dashboard/data-sources/Filebeat>`__.

::

   yum install systemd-devel || echo "This will only work on RHEL-based systems."
   apt-get install libsystemd-dev || echo "This will only work on Debian-based systems."

   go get github.com/mheese/journalbeat
   ansible-playbook -i inventory/digital_ocean.py -l sentrynet logzio.yml -e LOGZIO_TOKEN=ABCDEFGHIJKLMNOPQRSTUVWXYZ012345


