**OUTDATED**

# Using with lightclient

We have an awesome cluster running, let's try to test this out without
relying on executing commands on the cluster.  Rather, we can connect to the
rpc interface with the `light-client` package and execute commands locally,
or even proxy our webapp to the kubernetes backend.

## Setup

In order to get this working, we need to know a few pieces of info,
the chain id of tendermint, the chain id of basecoin, and an account
with a bit of cash....

### Tendermint Chain ID

`kubectl exec -c tm tm-0 -- curl -s http://tm-1.basecoin:26657/status | json_pp | grep network`

set TM_CHAIN with the value there

### Basecoin Chain ID

`kubectl exec -c app tm-1 -- grep -A1 chainID /app/genesis.json`

set BC_CHAIN with the value there

### Expose tendermint rpc

We need to be able to reach the tendermint rpc interface from our shell.

`kubectl port-forward tm-0 26657:26657`

### Start basecoin-proxy

Using this info, let's connect our proxy and get going

`proxy-basecoin -tmchain=$TM_CHAIN -chain=$BC_CHAIN -rpc=localhost:26657`

## Basecoin accounts

Well, we can connect, but we don't have a registered account yet...
Let's look around, then use the cli to send some money from one of
the validators to our client's address so we can play.

**TODO** we can add some of our known accounts (from `/keys`) into
the genesis file, so we can skip all the kubectl money fiddling here.
We will want to start with money on some known non-validators.

### Getting validator info (kubectl)

The basecoin app deployment starts with 1000 "blank" coin in an account of
each validator.  Let's get the address of the first validator

`kubectl exec -c app tm-1 -- grep address /app/key.json`

Store this info as VAL1_ADDR

### Querying state (proxy)

The proxy can read any public info via the tendermint rpc, so let's check
out this account.

`curl localhost:8108/query/account/$VAL1_ADDR`

Now, let's make out own account....

`curl -XPOST http://localhost:8108/keys/ -d '{"name": "k8demo", "passphrase": "1234567890"}'`

(or pick your own user and password).  Remember the address you get here.  You can
always find it out later by calling:

`curl http://localhost:8108/keys/k8demo`

and store it in DEMO_ADDR, which is empty at first

`curl localhost:8108/query/account/$DEMO_ADDR`


### "Stealing" validator cash (kubectl)

Run one command, that will be signed, now we have money

`kubectl exec -c app tm-0 -- basecoin tx send --to <k8demo-address> --amount 500`

### Using our money

Returning to our remote shell, we have a remote account with some money.
Let's see that.

`curl localhost:8108/query/account/$DEMO_ADDR`

Cool.  Now we need to send it to a second account.

`curl -XPOST http://localhost:8108/keys/ -d '{"name": "buddy", "passphrase": "1234567890"}'`

and store the resulting address in BUDDY_ADDR

**TODO** finish this

