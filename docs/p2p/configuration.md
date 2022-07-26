# Tendermint p2p configuration

This document contains configurable parameters a node operator can use to tune the p2p behaviour. 

(TODO List of parameters + the parts of the code and behaviour they impact)

| Parameter| Default| Note|
| --- | --- | ---|
|ListenAddress:  |              "tcp://0.0.0.0:26656" | |
|ExternalAddress:      |        "" | |
|	UPNP:             |             false, | |
|	AddrBook:          |           defaultAddrBookPath, | |
|	AddrBookStrict:     |          true,| |
|	MaxNumInboundPeers:  |         40,| |
|	MaxNumOutboundPeers:  |        10,| |
|	PersistentPeersMaxDialPeriod:| 0 * time.Second,| |
|	FlushThrottleTimeout:         |100 * time.Millisecond,| |
|	MaxPacketMsgPayloadSize:    |  1024,    // 1 kB |  |
|	SendRate:                    | 5120000, // 5 mB/s | |
|	RecvRate:            |         5120000, // 5 mB/s | |
|	PexReactor:           |        true, | |
|	SeedMode:              |       false, | |
|	AllowDuplicateIP:       |      false, | |
|	HandshakeTimeout:        |     20 * time.Second,| |
|	DialTimeout:              |    3 * time.Second,| |