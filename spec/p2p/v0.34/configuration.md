# Tendermint p2p configuration

This document contains configurable parameters a node operator can use to tune the p2p behaviour. 

(TODO List of parameters + the parts of the code and behaviour they impact)

| Parameter| Default| Note|
| --- | --- | ---|
|   ListenAddress  |              "tcp://0.0.0.0:26656" |   Address to listen for incoming connections  |
|   ExternalAddress      |        "" |  Address to advertise to peers for them to dial |
| Seeds | empty | Comma separated list of seed nodes to connect to| 
| Persistent peers | empty | Comma separated list of nodes to keep persistent connections to |
|	UPNP             |             false | UPNP port forwarding enabled |
|	AddrBook          |           defaultAddrBookPath, | Path do address book |
|	AddrBookStrict    |          true,| Set true for strict address routability rules and false for private or local networks |
|	MaxNumInboundPeers  |         40,| Maximum number of inbound peers |
|	MaxNumOutboundPeers  |        10,|  Maximum number of outbound peers to connect to, excluding persistent peers |
| UnconditionalPeers | empty | List of node IDs, to which a connection will be (re)established ignoring any existing limits |
|	PersistentPeersMaxDialPeriod| 0 * time.Second|Maximum pause when redialing a persistent peer (if zero, exponential backoff is used)  |
|	FlushThrottleTimeout         |100 * time.Millisecond | Time to wait before flushing messages out on the connection |
|	MaxPacketMsgPayloadSize    |  1024 | Maximum size of a message packet payload, in bytes |
|	SendRate                    | 5120000 (5 mB/s) |Rate at which packets can be sent, in bytes/second  |
|	RecvRate            |         5120000 (5 mB/s) | Rate at which packets can be received, in bytes/second|
|	PexReactor           |        true |Set true to enable the peer-exchange reactor |
|	SeedMode              |       false |Seed mode, in which node constantly crawls the network and looks for. Does not work if the peer-exchange reactor is disabled.  |
| PrivatePeerIDs | empty | Comma separated list of peer IDs to keep private (will not be gossiped to other peers)|
|	AllowDuplicateIP       |      false | Toggle to disable guard against peers connecting from the same ip.|
|	HandshakeTimeout        |     20 * time.Second| Peer connection configuration. |
|	DialTimeout:              |    3 * time.Second| |