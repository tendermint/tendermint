Deploy a Testnet
================

Now that we've seen how ABCI works, and even played with a few
applications on a single validator node, it's time to deploy a test
network to four validator nodes. For this deployment, we'll use the
``basecoin`` application.

Manual Deployments
------------------

It's relatively easy to setup a Tendermint cluster manually. The only
requirements for a particular Tendermint node are a private key for the
validator, stored as ``priv_validator.json``, and a list of the public
keys of all validators, stored as ``genesis.json``. These files should
be stored in ``~/.tendermint``, or wherever the ``$TMROOT`` variable
might be set to.

Here are the steps to setting up a testnet manually:

1) Provision nodes on your cloud provider of choice
2) Install Tendermint and the application of interest on all nodes
3) Generate a private key for each validator using
   ``tendermint gen_validator``
4) Compile a list of public keys for each validator into a
   ``genesis.json`` file.
5) Run ``tendermint node --p2p.seeds=< seed addresses >`` on each node,
   where ``< seed addresses >`` is a comma separated list of the IP:PORT
   combination for each node. The default port for Tendermint is
   ``46656``. Thus, if the IP addresses of your nodes were
   ``192.168.0.1, 192.168.0.2, 192.168.0.3, 192.168.0.4``, the command
   would look like:
   ``tendermint node --p2p.seeds=192.168.0.1:46656,192.168.0.2:46656,192.168.0.3:46656,192.168.0.4:46656``.

After a few seconds, all the nodes should connect to eachother and start
making blocks! For more information, see the Tendermint Networks section
of `the guide to using Tendermint <using-tendermint.html>`__.

Automated Deployments
---------------------

While the manual deployment is easy enough, an automated deployment is
always better. For this, we have the `mintnet-kubernetes
tool <https://github.com/tendermint/tools/tree/master/mintnet-kubernetes>`__,
which allows us to automate the deployment of a Tendermint network on an
already provisioned kubernetes cluster. And for simple provisioning of kubernetes
cluster, check out the `Google Cloud Platform <https://cloud.google.com/>`__.
