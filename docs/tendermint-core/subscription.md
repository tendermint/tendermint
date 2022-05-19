---
order: 7
---

# Subscribing to Events

A Tendermint node emits events about important state transitions during
consensus. These events can be queried by clients via the [RPC interface][rpc]
on nodes that enable it. The [list of supported event types][event-types] can
be found in the tendermint/types Go package.

In Tendermint v0.36 there are two APIs to query events:

- The [**legacy streaming API**](#legacy-streaming-api), comprising the
  `subscribe`, `unsubscribe`, and `unsubscribe_all` RPC methods over websocket.

- The [**event log API**](#event-log-api), comprising the `events` RPC method.

The legacy streaming API is deprecated in Tendermint v0.36, and will be removed
in Tendermint v0.37. Clients are strongly encouraged to migrate to the new
event log API as soon as is practical.

[rpc]: https://docs.tendermint.com/master/rpc
[event-types]: https://godoc.org/github.com/tendermint/tendermint/types#EventNewBlockValue

## Filter Queries

Event requests take a [filter query][query] parameter. A filter query is a
string that describes a subset of available event items to return.  An empty
query matches all events; otherwise a query comprises one or more *terms*
comparing event metadata to target values.

For example, to select new block events, use the term:

```
tm.event = 'NewBlock'
```

Multiple terms can be combined with `AND` (case matters), for example to match
the transaction event with a given hash, use the query:

```
tm.event = 'Tx' AND tx.hash = 'EA7B33F'
```

Operands may be strings in single quotes (`'Tx'`), numbers (`45`), dates, or
timestamps.

The comparison operators include `=`, `<`, `<=`, `>`, `>=`, and `CONTAINS` (for
substring match).  In addition, the `EXISTS` operator checks for the presence
of an attribute regardless of its value.

### Attributes

Tendermint implicitly defines a string-valued `tm.event` attribute for all
event types. Transaction items (type `Tx`) are also assigned `tx.hash`
(string), giving the hash of the transaction, and and `tx.height` (number)
giving the height of the block containing the transaction.  For `NewBlock` and
`NewBlockHeader` events, Tendermint defines a `block.height` attribute giving
the height of the block.

Additional attributes can be provided by the application as [ABCI `Event`
records][abci-event] in response to the `FinalizeBlock` request.  The full name
of the attribute in the query is formed by combining the `type` and attribute
`key` with a period.

For example, given the events

```go
[]abci.Event{{
    Type: "reward",
    Attributes: []abci.EventAttribute{
        {Key: "address", Value: "cosmos1xyz012pdq"},
        {Key: "amount", Value: "45.62"},
        {Key: "balance", Value: "100.390001"},
    },
}}
```

a query may refer to the names `reward.address`, `reward.amount`, and `reward.balance`, as in:

```
reward.address EXISTS AND reward.balance > 45
```

Certain application-specific metadata are also indexed for offline queries.
See [Indexing transactions](../app-dev/indexing-transactions.md) for more details.

[query]: https://godoc.org/github.com/tendermint/tendermint/internal/pubsub/query/syntax
[abci-event]: https://github.com/tendermint/tendermint/blob/master/proto/tendermint/abci/types.proto#L397

## Event Log API

Starting in Tendermint v0.36, when the `rpc.event-log-window-size`
configuration is enabled, the node maintains maintains a log of all events
within this operator-defined time window. This API supersedes the websocket
subscription API described below.

Clients can query these events can by long-polling the `/events` RPC method,
which returns the most recent items from the log that match the [request
parameters][reqevents].  Each item returned includes a cursor that marks its
location in the log. Cursors can be passed via the `before` and `after`
parameters to fetch events earlier in the log.

For example, this request:

```json
{
  "jsonrpc": "2.0",
  "id": 1,
  "method": "events",
  "params": {
    "filter": {
      "query": "tm.event = 'Tx' AND app.key = 'applesauce'"
    },
    "maxItems": 1,
    "after": ""
  }
}
```

will return a result similar to the following:

```json
{
  "jsonrpc": "2.0",
  "id": 1,
  "result": {
    "items": [
      {
        "cursor": "16ee3d5e65be53d8-03d5",
        "event": "Tx",
        "data": {
          "type": "tendermint/event/Tx",
          "value": {
            "height": 70,
            "tx": "YXBwbGVzYXVjZT1zeXJ1cA==",
            "result": {
              "events": [
                {
                  "type": "app",
                  "attributes": [
                    {
                      "key": "creator",
                      "value": "Cosmoshi Netowoko",
                      "index": true
                    },
                    {
                      "key": "key",
                      "value": "applesauce",
                      "index": true
                    },
                    {
                      "key": "index_key",
                      "value": "index is working",
                      "index": true
                    },
                    {
                      "key": "noindex_key",
                      "value": "index is working",
                      "index": false
                    }
                  ]
                }
              ]
            }
          }
        }
      }
    ],
    "more": false,
    "oldest": "16ee3d4c471c3b00-0001",
    "newest": "16ee3d5f2e05a4e0-0400"
  }
}
```

The `"items"` array gives the matching items (up to the requested
`"maxResults"`) in decreasing time order (i.e., newest to oldest).  In this
case, there is only one result, but if there are additional results that were
not returned, the `"more"` flag will be true. Calling `/events` again with the
same query and `"after"` set to the cursor of the newest result (in this
example, `"16ee3d5e65be53d8-03d5"`) will fetch newer results.

Go clients can use the [`eventstream`][eventstream] package to simplify the use
of this method. The `eventstream.Stream` automatically handles polling for new
events, updating the cursor, and reporting any missed events.

[reqevents]: https://pkg.go.dev/github.com/tendermint/tendermint@master/rpc/coretypes#RequestEvents
[eventstream]: https://godoc.org/github.com/tendermint/tendermint/rpc/client/eventstream

## Legacy Streaming API

- **Note:** This API is deprecated in Tendermint v0.36, and will be removed in
  Tendermint v0.37. New clients and existing use should use the [event log
  API](#event-log-api) instead. See [ADR 075][adr075] for more details.

To subscribe to events in the streaming API, you must connect to the node RPC
service using a [websocket][ws]. From the command line you can use a tool such
as [wscat][wscat], for example:

```sh
wscat ws://127.0.0.1:26657/websocket
```

[ws]: https://en.wikipedia.org/wiki/WebSocket
[wscat]: https://github.com/websockets/wscat

To subscribe to events, call the `subscribe` JSON-RPC method method passing in
a [filter query][query] for the events you wish to receive:

```json
{
  "jsonrpc": "2.0",
  "method": "subscribe",
  "id": 1,
  "params": {
    "query": "tm.event='NewBlock'"
  }
}
```

The subscribe method returns an initial response confirming the subscription,
then sends additional JSON-RPC response messages containing the matching events
as they are published. The subscription continues until either the client
explicitly cancels the subscription (by calling `unsubscribe` or
`unsubscribe_all`) or until the websocket connection is terminated.

[adr075]: https://tinyurl.com/adr075
