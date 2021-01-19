---
order: 6
---

# Peer Exchange

## Channels

Pex has one channel. The channel identifier is listed below.

| Name       | Number |
|------------|--------|
| PexChannel | 0      |

## Message Types

### PexRequest

PexRequest is an empty message requesting a list of peers.

> EmptyRequest

### PexAddrs

PexAddrs is an list of net addresses provided to a peer to dial.

| Name  | Type                               | Description                              | Field Number |
|-------|------------------------------------|------------------------------------------|--------------|
| addrs | repeated [NetAddress](#netaddress) | List of peer addresses available to dial | 1            |

### NetAddress

NetAddress provides needed information for a node to dial a peer.

| Name | Type   | Description      | Field Number |
|------|--------|------------------|--------------|
| id   | string | NodeID of a peer | 1            |
| ip   | string | The IP of a node | 2            |
| port | port   | Port of a peer   | 3            |

### Message

Message is a [`oneof` protobuf type](https://developers.google.com/protocol-buffers/docs/proto#oneof). The one of consists of two messages.

| Name        | Type                      | Description                                          | Field Number |
|-------------|---------------------------|------------------------------------------------------|--------------|
| pex_request | [PexRequest](#pexrequest) | Empty request asking for a list of addresses to dial | 1            |
| pex_addrs   | [PexAddrs]                | List of addresses to dial                            | 2            |
