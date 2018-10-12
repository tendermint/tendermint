# Deploy a Testnet

DEPRECATED DOCS!

See [Networks](../networks).

## Manual Deployments

It's relatively easy to setup a Tendermint cluster manually. The only
requirements for a particular Tendermint node are a private key for the
validator, stored as `priv_validator.json`, a node key, stored as
`node_key.json` and a list of the public keys of all validators, stored
as `genesis.json`. These files should be stored in
`~/.tendermint/config`, or wherever the `$TMHOME` variable might be set
to.

Here are the steps to setting up a testnet manually:

1.  Provision nodes on your cloud provider of choice
2.  Install Tendermint and the application of interest on all nodes
3.  Generate a private key and a node key for each validator using
    `tendermint init`
4.  Compile a list of public keys for each validator into a
    new `genesis.json` file and replace the existing file with it.
5.  Get the node IDs of any peers you want other peers to connect to by
    running `tendermint show_node_id` on the relevant machine
6.  Set the `p2p.persistent_peers` in the config for all nodes to the comma
    separated list of `ID@IP:PORT` for all nodes. Default port is 26656.

Then start the node

```
tendermint node --proxy_app=kvstore
```

After a few seconds, all the nodes should connect to each other and
start making blocks! For more information, see the Tendermint Networks
section of [the guide to using Tendermint](../tendermint-core/using-tendermint.md).

But wait! Steps 3, 4 and 5 are quite manual. Instead, use the `tendermint testnet` command. By default, running `tendermint testnet` will create all the
required files, but it won't populate the list of persistent peers. It will do
it however if you provide the `--populate-persistent-peers` flag and optional
`--starting-ip-address` flag. Run `tendermint testnet --help` for more details
on the available flags.

```
tendermint testnet --populate-persistent-peers --starting-ip-address 192.168.0.1
```

This command will generate four folders, prefixed with "node" and put them into
the "./mytestnet" directory by default.

As you might imagine, this command is useful for manual or automated
deployments.

## Automated Deployments

The easiest and fastest way to get a testnet up in less than 5 minutes.

### Local

With `docker` and `docker-compose` installed, run the command:

```
make localnet-start
```

from the root of the tendermint repository. This will spin up a 4-node
local testnet. Note that this command expects a linux binary in the build directory. 
If you built the binary using a non-linux OS, you may see 
the error `Binary needs to be OS linux, ARCH amd64`, in which case you can
run:

```
make build-linux
make localnet-start
```

Review the target in the Makefile to debug any problems.

### Cloud

See the [next section](./terraform-and-ansible.md) for details.
