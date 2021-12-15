---
order: 2
---

# Debug Like A Pro

## Intro

Tendermint Core is a fairly robust BFT replication engine. Unfortunately, as with other software, failures sometimes do happen. The question is then “what do you do” when the system deviates from the expected behavior.

The first response is usually to take a look at the logs. By default, Tendermint writes logs to standard output ¹.

```sh
I[2020-05-29|03:03:16.145] Committed state                              module=state height=2282 txs=0 appHash=0A27BC6B0477A8A50431704D2FB90DB99CBFCB67A2924B5FBF6D4E78538B67C1I[2020-05-29|03:03:21.690] Executed block                               module=state height=2283 validTxs=0 invalidTxs=0I[2020-05-29|03:03:21.698] Committed state                              module=state height=2283 txs=0 appHash=EB4E409D3AF4095A0757C806BF160B3DE4047AC0416F584BFF78FC0D44C44BF3I[2020-05-29|03:03:27.994] Executed block                               module=state height=2284 validTxs=0 invalidTxs=0I[2020-05-29|03:03:28.003] Committed state                              module=state height=2284 txs=0 appHash=3FC9237718243A2CAEE3A8B03AE05E1FC3CA28AEFE8DF0D3D3DCE00D87462866E[2020-05-29|03:03:32.975] enterPrevote: ProposalBlock is invalid       module=consensus height=2285 round=0 err="wrong signature (#35): C683341000384EA00A345F9DB9608292F65EE83B51752C0A375A9FCFC2BD895E0792A0727925845DC13BA0E208C38B7B12B2218B2FE29B6D9135C53D7F253D05"
```

If you’re running a validator in production, it might be a good idea to forward the logs for analysis using filebeat or similar tools. Also, you can set up a notification in case of any errors.

The logs should give you the basic idea of what has happened. In the worst-case scenario, the node has stalled and does not produce any logs (or simply panicked).

The next step is to call /status, /net_info, /consensus_state and /dump_consensus_state RPC endpoints.

```sh
curl http://<server>:26657/status$ curl http://<server>:26657/net_info$ curl http://<server>:26657/consensus_state$ curl http://<server>:26657/dump_consensus_state
```

Please note that /consensus_state and /dump_consensus_state may not return a result if the node has stalled (since they try to get a hold of the consensus mutex).

The output of these endpoints contains all the information needed for developers to understand the state of the node. It will give you an idea if the node is lagging behind the network, how many peers it’s connected to, and what the latest consensus state is.

At this point, if the node is stalled and you want to restart it, the best thing you can do is to kill it with -6 signal:

```sh
kill -6 <PID>
```

which will dump the list of the currently running goroutines. The list is super useful when debugging a deadlock.

`PID` is the Tendermint’s process ID. You can find it out by running `ps -a | grep tendermint | awk ‘{print $1}’`

## Tendermint debug kill

To ease the burden of collecting different pieces of data Tendermint Core (since v0.33 version) provides the Tendermint debug kill tool, which will do all of the above steps for you, wrapping everything into a nice archive file.

```sh
tendermint debug kill <pid> </path/to/out.zip> — home=</path/to/app.d>
```

Here’s the official documentation page — <https://docs.tendermint.com/master/tools/debugging>

If you’re using a process supervisor, like systemd, it will restart the Tendermint automatically. We strongly advise you to have one in production. If not, you will need to restart the node by hand.

Another advantage of using Tendermint debug is that the same archive file can be given to Tendermint Core developers, in cases where you think there’s a software issue.

## Tendermint debug dump

Okay, but what if the node has not stalled, but its state is degrading over time? Tendermint debug dump to the rescue!

```sh
tendermint debug dump </path/to/out> — home=</path/to/app.d>
```

It won’t kill the node, but it will gather all of the above data and package it into an archive file. Plus, it will also make a heap dump, which should help if Tendermint is leaking memory.

At this point, depending on how severe the degradation is, you may want to restart the process.

## Tendermint Inspect

What if the Tendermint node will not start up due to inconsistent consensus state? 

When a node running the Tendermint consensus engine detects an inconsistent state 
it will crash the entire Tendermint process. 
The Tendermint consensus engine cannot be run in this inconsistent state and the so node
will fail to start up as a result.
The Tendermint RPC server can provide valuable information for debugging in this situation.
The Tendermint `inspect` command will run a subset of the Tendermint RPC server 
that is useful for debugging inconsistent state.

### Running inspect

Start up the `inspect` tool on the machine where Tendermint crashed using: 
```bash
tendermint inspect --home=</path/to/app.d>
```

`inspect` will use the data directory specified in your Tendermint configuration file.
`inspect` will also run the RPC server at the address specified in your Tendermint configuration file.

### Using inspect

With the `inspect` server running, you can access RPC endpoints that are critically important
for debugging.
Calling the `/status`, `/consensus_state` and `/dump_consensus_state` RPC endpoint 
will return useful information about the Tendermint consensus state.

## Outro

We’re hoping that these Tendermint tools will become de facto the first response for any accidents.

Let us know what your experience has been so far! Have you had a chance to try `tendermint debug` or `tendermint inspect` yet?

Join our [discord chat](https://discord.gg/cosmosnetwork), where we discuss the current issues and future improvements.

—

[1]: Of course, you’re free to redirect the Tendermint’s output to a file or forward it to another server.
