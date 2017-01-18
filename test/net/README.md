# Stack

This is a stripped down version of https://github.com/segmentio/stack
plus some shell scripts.

It is responsible for the following:

	- spin up a cluster of nodes
	- copy config files for a tendermint testnet to each node
	- copy linux binaries for tendermint and the app to each node
	- start tendermint on every node

# How it Works

To use, a user must only provide a directory containing two files: `bins` and `run.sh`.

The `bins` file is a list of binaries, for instance:

```
$GOPATH/bin/tendermint
$GOPATH/bin/dummy
```

and the `run.sh` specifies how those binaries ought to be started:

```
#! /bin/bash

if [[ "$SEEDS" != "" ]]; then
	SEEDS_FLAG="--seeds=$SEEDS"
fi

./dummy --persist .tendermint/data/dummy_data >> app.log 2>&1 &
./tendermint node --log_level=info $SEEDS_FLAG >> tendermint.log 2>&1 &
```

This let's you specify exactly which versions of Tendermint and the application are to be used,
and how they ought to be started.

Note that these binaries *MUST* be compiled for Linux. 
If you are not on Linux, you can compile binaries for linux using `go build` with the `GOOS` variable:

```
GOOS=linux go build -o $GOPATH/bin/tendermint-linux $GOPATH/src/github.com/tendermint/tendermint/cmd/tendermint
```

This cross-compilation must be done for each binary you want to copy over. 

If you want to use an application that requires more than just a few binaries, you may need to do more manual work, 
for instance using `terraforce` to set up the development environment on every machine.

# Dependencies

We use `terraform` for spinning up the machines, 
and a custom rolled tool, `terraforce`, 
for running commands on many machines in parallel.
You can download terraform here: https://www.terraform.io/downloads.html
To download terraforce, run `go get github.com/ebuchman/terraforce`

We use `tendermint` itself to generate files for a testnet.
You can install `tendermint` with 

```
cd $GOPATH/src/github.com/tendermint/tendermint
glide install
go install ./cmd/tendermint
```

You also need to set the `DIGITALOCEAN_TOKEN` environment variables so that terraform can 
spin up nodes on digital ocean.

This stack is currently some terraform and a bunch of shell scripts, 
so its helpful to work out of a directory containing everything. 
Either 	change directory to `$GOPATH/src/github.com/tendermint/tendermint/test/net`
or make a copy of that directory and change to it. All commands are expected to be executed from there.

For terraform to work, you must first run `terraform get`

# Create 

To create a cluster with 4 nodes, run

```
terraform apply
```

To use a different number of nodes, change the `desired_capacity` parameter in the `main.tf`.

Note that terraform keeps track of the current state of your infrastructure, 
so if you change the `desired_capacity` and run `terraform apply` again, it will add or remove nodes as necessary.

If you think that's amazing, so do we.

To get some info about the cluster, run `terraform output`.

See the [terraform docs](https://www.terraform.io/docs/index.html) for more details. 

To tear down the cluster, run `terraform destroy`.

# Initialize 

Now that we have a cluster up and running, let's generate the necessary files for a Tendermint node and copy them over.
A Tendermint node needs, at the least, a `priv_validator.json` and a `genesis.json`.
To generate files for the nodes, run

```
tendermint testnet 4 mytestnet
```

This will create the directory `mytestnet`, containing one directory for each of the 4 nodes.
Each node directory contains a unique `priv_validator.json` and a `genesis.json`, 
where the `genesis.json` contains the public keys of all `priv_validator.json` files.

If you want to add more files to each node for your particular app, you'll have to add them to each of the node directories.

Now we can copy everything over to the cluster.
If you are on Linux, run 

```
bash scripts/init.sh 4 mytestnet examples/in-proc
```

Otherwise (if you are not on Linux), make sure you ran 

```
GOOS=linux go build -o $GOPATH/bin/tendermint-linux $GOPATH/src/github.com/tendermint/tendermint/cmd/tendermint
```

and now run 

```
bash scripts/init.sh 4 mytestnet examples/in-proc-linux
```

# Start 

Finally, to start Tendermint on all the nodes, run

```
bash scripts/start.sh 4
```

# Check

Query the status of all your nodes:

```
bash scripts/query.sh 4 status
```
