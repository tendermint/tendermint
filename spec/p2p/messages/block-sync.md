---
order: 2
---

# Block Sync

## Channel

Block sync has one channel.

| Name              | Number |
|-------------------|--------|
| BlockchainChannel | 64     |

## Message Types

There are multiple message types for Block Sync

### BlockRequest

BlockRequest asks a peer for a block at the height specified.

| Name   | Type  | Description               | Field Number |
|--------|-------|---------------------------|--------------|
| Height | int64 | Height of requested block | 1            |

### NoBlockResponse

NoBlockResponse notifies the peer requesting a block that the node does not contain it.

| Name   | Type  | Description               | Field Number |
|--------|-------|---------------------------|--------------|
| Height | int64 | Height of requested block | 1            |

### BlockResponse

BlockResponse contains the block requested.

| Name  | Type                                         | Description     | Field Number |
|-------|----------------------------------------------|-----------------|--------------|
| Block | [Block](../../core/data_structures.md#block) | Requested Block | 1            |

### StatusRequest

StatusRequest is an empty message that notifies the peer to respond with the highest and lowest blocks it has stored.

> Empty message.

### StatusResponse

StatusResponse responds to a peer with the highest and lowest block stored.

| Name   | Type  | Description                                                       | Field Number |
|--------|-------|-------------------------------------------------------------------|--------------|
| Height | int64 | Current Height of a node                                          | 1            |
| base   | int64 | First known block, if pruning is enabled it will be higher than 1 | 1            |

### Message

Message is a [`oneof` protobuf type](https://developers.google.com/protocol-buffers/docs/proto#oneof). The `oneof` consists of five messages.

| Name              | Type                             | Description                                                  | Field Number |
|-------------------|----------------------------------|--------------------------------------------------------------|--------------|
| block_request     | [BlockRequest](#blockrequest)    | Request a block from a peer                                  | 1            |
| no_block_response | [NoBlockResponse](#noblockresponse) | Response saying it doe snot have the requested block         | 2            |
| block_response    | [BlockResponse](#blockresponse)   | Response with requested block                                | 3            |
| status_request    | [StatusRequest](#statusrequest)   | Request the highest and lowest block numbers from a peer     | 4            |
| status_response   | [StatusResponse](#statusresponse)  | Response with the highest and lowest block numbers the store | 5            |
