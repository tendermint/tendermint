# ADR 033: pubsub 2.0

Author: Anton Kaliaev (@melekes)

## Changelog

02-10-2018: Initial draft

## Context

Since the initial version of the pubsub, there's been a number of issues
raised: #951, #1879, #1880. Some of them are high-level issues questioning the
core design choices made. Others are minor and mostly about the interface of
`Subscribe()` / `Publish()` functions.

### Sync vs Async

Now, when publishing a message to subscribers, we can do it in a goroutine:

_using channels for data transmission_
```go
for each subscriber {
    out := subscriber.outc
    go func() {
        out <- msg
    }
}
```

_by invoking callback functions_
```go
for each subscriber {
    go subscriber.callbackFn()
}
```

This gives us greater performance and allows us to avoid "slow client problem"
(when other subscribers have to wait for a slow subscriber). A pool of
goroutines can be used to avoid uncontrolled memory growth.

In certain cases, this is what you want. But in our case, because we need
strict ordering of events (if event A was published before B, the guaranteed
delivery order will be A -> B), we can't use goroutines.

There is also a question whenever we should have a non-blocking send:

```go
for each subscriber {
    out := subscriber.outc
    select {
        case out <- msg:
        default:
            log("subscriber %v buffer is full, skipping...")
    }
}
```

This fixes the "slow client problem", but there is no way for a slow client to
know if it had missed a message. On the other hand, if we're going to stick
with blocking send, **devs must always ensure subscriber's handling code does not
block**. As you can see, there is an implicit choice between ordering guarantees
and using goroutines.

The interim option is to run goroutines pool for a single message, wait for all
goroutines to finish. This will solve "slow client problem", but we'd still
have to wait `max(goroutine_X_time)` before we can publish the next message.
My opinion: not worth doing.

### Channels vs Callbacks

Yet another question is whether we should use channels for message transmission or
call subscriber-defined callback functions. Callback functions give subscribers
more flexibility - you can use mutexes in there, channels, spawn goroutines,
anything you really want. But they also carry local scope, which can result in
memory leaks and/or memory usage increase.

Go channels are de-facto standard for carrying data between goroutines.

**Question: Is it worth switching to callback functions?**

### Why `Subscribe()` accepts an `out` channel?

Because in our tests, we create buffered channels (cap: 1). Alternatively, we
can make capacity an argument.

## Decision

Change Subscribe() function to return out channel:

```go
// outCap can be used to set capacity of out channel (unbuffered by default).
Subscribe(ctx context.Context, clientID string, query Query, outCap... int) (out <-chan interface{}, err error) {
```

It's more idiomatic since we're closing it during Unsubscribe/UnsubscribeAll calls.

Also, we should make tags available to subscribers:

```go
type MsgAndTags struct {
    Msg interface{}
    Tags TagMap
}

// outCap can be used to set capacity of out channel (unbuffered by default).
Subscribe(ctx context.Context, clientID string, query Query, outCap... int) (out <-chan MsgAndTags, err error) {
```

## Status

In review

## Consequences

### Positive

- more idiomatic interface
- subscribers know what tags msg was published with

### Negative

### Neutral
