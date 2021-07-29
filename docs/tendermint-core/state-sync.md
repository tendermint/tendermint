--- 
order: 11
---

# State Sync

With block sync a node is downloading all of the data of an application from genesis and verifying it. 
With state sync your node will download data related to the head or near the head of the chain and verify the data. 
This leads to drastically shorter times for joining a network. 

Information on how to configure state sync is located in the [nodes section](../nodes/state-sync.md)

## Events

When a node starts with the statesync flag enabled in the config file, it will emit two events: one upon starting statesync and the other upon completion.

The user can query the events by subscribing `EventQueryStateSyncStatus`
Please check [types](https://pkg.go.dev/github.com/tendermint/tendermint/types?utm_source=godoc#pkg-constants) for the details.