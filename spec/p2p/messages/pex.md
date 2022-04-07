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

### PexResponse

PexResponse is an list of net addresses provided to a peer to dial.

| Name  | Type                               | Description                              | Field Number |
|-------|------------------------------------|------------------------------------------|--------------|
| addresses | repeated [PexAddress](#pexaddress) | List of peer addresses available to dial | 1            |

### PexAddress

PexAddress provides needed information for a node to dial a peer. This is in the form of a `URL` that gets parsed
into a `NodeAddress`. See [ParseNodeAddress](https://github.com/tendermint/tendermint/blob/f2a8f5e054cf99ebe246818bb6d71f41f9a30faa/internal/p2p/address.go#L43) for more details.

| Name | Type   | Description      | Field Number |
|------|--------|------------------|--------------|
| url   | string | See [golang url](https://golang.org/pkg/net/url/#URL) | 1            |

### Message

Message is a [`oneof` protobuf type](https://developers.google.com/protocol-buffers/docs/proto#oneof). The one of consists of two messages.

| Name         | Type                        | Description                                          | Field Number |
|--------------|-----------------------------|------------------------------------------------------|--------------|
| pex_request  | [PexRequest](#pexrequest)   | Empty request asking for a list of addresses to dial | 3            |
| pex_response | [PexResponse](#pexresponse) | List of addresses to dial                            | 4            |
