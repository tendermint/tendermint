---
order: 1
parent:
  title: Peer Exchange
  order: 5
---

# Peer Strategy and Exchange

Here we outline the design of the PeerStore
and how it used by the Peer Exchange Reactor (PEX) to ensure we are connected
to good peers and to gossip peers to others.

## Peer Types

Certain peers are special in that they are specified by the user as `persistent`,
which means we auto-redial them if the connection fails, or if we fail to dial
them.
Some peers can be marked as `private`, which means
we will not put them in the peer store or gossip them to others.

All peers except private peers and peers coming from them are tracked using the
peer store.

The rest of our peers are only distinguished by being either
inbound (they dialed our public address) or outbound (we dialed them).

## Discovery

Peer discovery begins with a list of seeds.

When we don't have enough peers, we

1. ask existing peers
2. dial seeds if we're not dialing anyone currently

On startup, we will also immediately dial the given list of `persistent_peers`,
and will attempt to maintain persistent connections with them. If the
connections die, or we fail to dial, we will redial every 5s for a few minutes,
then switch to an exponential backoff schedule, and after about a day of
trying, stop dialing the peer. This behavior is when `persistent_peers_max_dial_period` is configured to zero.

But If `persistent_peers_max_dial_period` is set greater than zero, terms between each dial to each persistent peer
will not exceed `persistent_peers_max_dial_period` during exponential backoff.
Therefore, `dial_period` = min(`persistent_peers_max_dial_period`, `exponential_backoff_dial_period`)
and we keep trying again regardless of `maxAttemptsToDial`

As long as we have less than `MaxNumOutboundPeers`, we periodically request
additional peers from each of our own and try seeds.

## Listening

Peers listen on a configurable ListenAddr that they self-report in their
NodeInfo during handshakes with other peers. Peers accept up to
`MaxNumInboundPeers` incoming peers.

## Address Book

Peers are tracked via their ID (their PubKey.Address()).
Peers are added to the peer store from the PEX when they first connect to us or
when we hear about them from other peers.

The peer store is arranged in sets of buckets, and distinguishes between
vetted (old) and unvetted (new) peers. It keeps different sets of buckets for
vetted and unvetted peers. Buckets provide randomization over peer selection.
Peers are put in buckets according to their IP groups.

IP group can be a masked IP (e.g. `1.2.0.0` or `2602:100::`) or `local` for
local addresses or `unroutable` for unroutable addresses. The mask which
corresponds to the `/16` subnet is used for IPv4, `/32` subnet - for IPv6.
Each group has a limited number of buckets to prevent DoS attacks coming from
that group (e.g. an attacker buying a `/16` block of IPs and launching a DoS
attack).

[highwayhash](https://arxiv.org/abs/1612.06257) is used as a hashing function
when calculating a bucket.

When placing a peer into a new bucket:

```md
hash(key + sourcegroup + int64(hash(key + group + sourcegroup)) % bucket_per_group) % num_new_buckets
```

When placing a peer into an old bucket:

```md
hash(key + group + int64(hash(key + addr)) % buckets_per_group) % num_old_buckets
```

where `key` - random 24 HEX string, `group` - IP group of the peer (e.g. `1.2.0.0`),
`sourcegroup` - IP group of the sender (peer who sent us this address) (e.g. `174.11.0.0`),
`addr` - string representation of the peer's address (e.g. `174.11.10.2:26656`).

A vetted peer can only be in one bucket. An unvetted peer can be in multiple buckets, and
each instance of the peer can have a different IP:PORT.

If we're trying to add a new peer but there's no space in its bucket, we'll
remove the worst peer from that bucket to make room.

## Vetting

When a peer is first added, it is unvetted.
Marking a peer as vetted is outside the scope of the `p2p` package.
For Tendermint, a Peer becomes vetted once it has contributed sufficiently
at the consensus layer; ie. once it has sent us valid and not-yet-known
votes and/or block parts for `NumBlocksForVetted` blocks.
Other users of the p2p package can determine their own conditions for when a peer is marked vetted.

If a peer becomes vetted but there are already too many vetted peers,
a randomly selected one of the vetted peers becomes unvetted.

If a peer becomes unvetted (either a new peer, or one that was previously vetted),
a randomly selected one of the unvetted peers is removed from the peer store.

More fine-grained tracking of peer behaviour can be done using
a trust metric (see below), but it's best to start with something simple.

## Select Peers to Dial

When we need more peers, we pick addresses randomly from the addrbook with some
configurable bias for unvetted peers. The bias should be lower when we have
fewer peers and can increase as we obtain more, ensuring that our first peers
are more trustworthy, but always giving us the chance to discover new good
peers.

We track the last time we dialed a peer and the number of unsuccessful attempts
we've made. If too many attempts are made, we mark the peer as bad.

Connection attempts are made with exponential backoff (plus jitter). Because
the selection process happens every `ensurePeersPeriod`, we might not end up
dialing a peer for much longer than the backoff duration.

If we fail to connect to the peer after 16 tries (with exponential backoff), we
remove from peer store completely. But for persistent peers, we indefinitely try to
dial all persistent peers unless `persistent_peers_max_dial_period` is configured to zero

## Select Peers to Exchange

When we’re asked for peers, we select them as follows:

- select at most `maxGetSelection` peers
- try to select at least `minGetSelection` peers - if we have less than that, select them all.
- select a random, unbiased `getSelectionPercent` of the peers

Send the selected peers. Note we select peers for sending without bias for vetted/unvetted.

## Preventing Spam

There are various cases where we decide a peer has misbehaved and we disconnect from them.
When this happens, the peer is removed from the peer store and black listed for
some amount of time. We call this "Disconnect and Mark".
Note that the bad behaviour may be detected outside the PEX reactor itself
(for instance, in the mconnection, or another reactor), but it must be communicated to the PEX reactor
so it can remove and mark the peer.

In the PEX, if a peer sends us an unsolicited list of peers,
or if the peer sends a request too soon after another one,
we Disconnect and MarkBad.

## Trust Metric

The quality of peers can be tracked in more fine-grained detail using a
Proportional-Integral-Derivative (PID) controller that incorporates
current, past, and rate-of-change data to inform peer quality.

While a PID trust metric has been implemented, it remains for future work
to use it in the PEX.

See the [trustmetric](https://github.com/tendermint/tendermint/blob/master/docs/architecture/adr-006-trust-metric.md)
and [trustmetric useage](https://github.com/tendermint/tendermint/blob/master/docs/architecture/adr-007-trust-metric-usage.md)
architecture docs for more details.





<!-- todo: diagrams!!! -->
