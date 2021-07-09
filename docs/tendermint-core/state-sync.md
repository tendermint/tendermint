--- 
order: 11
---

# State Sync

With fast sync a node is downloading all of the data of an application from genesis and verifying it. 
With state sync your node will download data related to the head or near the head of the chain and verify the data. 
This leads to drastically shorter times for joining a network. 

Information on how to configure state sync is located in the [nodes section](../nodes/state-sync.md)

## The State Sync event
When the tendermint blockchain core launches and the statesync flag has been enabled in the config file,
it might start the state-sync to fetch the chain state data from the network. During the starting / finished
the data fetching progress, it will emit an event to expose the state-sync `complete` status and the state `height`.  

The user can query the events by subscribing `EventQueryStateSyncStatus`
Please check [types](https://pkg.go.dev/github.com/tendermint/tendermint/types?utm_source=godoc#pkg-constants) for the details.