# Peer Strategy and Exchange

Here we outline the design of the AddressBook
and how it used by the Peer Exchange Reactor (PEX) to ensure we are connected
to good peers and to gossip peers to others.

## Peer Types

Certain peers are special in that they are specified by the user as `persistent`,
which means we auto-redial them if the connection fails.
Some peers can be marked as `private`, which means
we will not put them in the address book or gossip them to others.

All peers except private peers are tracked using the address book.

## Discovery

Peer discovery begins with a list of seeds.
When we have no peers, or have been unable to find enough peers from existing ones,
we dial a randomly selected seed to get a list of peers to dial.

So long as we have less than `MaxPeers`, we periodically request additional peers
from each of our own. If sufficient time goes by and we still can't find enough peers,
we try the seeds again.

## Address Book

Peers are tracked via their ID (their PubKey.Address()).
For each ID, the address book keeps the most recent IP:PORT.
Peers are added to the address book from the PEX when they first connect to us or
when we hear about them from other peers.

The address book is arranged in sets of buckets, and distinguishes between
vetted (old) and unvetted (new) peers. It keeps different sets of buckets for vetted and
unvetted peers. Buckets provide randomization over peer selection.

A vetted peer can only be in one bucket. An unvetted peer can be in multiple buckets.

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
a randomly selected one of the unvetted peers is removed from the address book.

More fine-grained tracking of peer behaviour can be done using
a trust metric (see below), but it's best to start with something simple.

## Select Peers to Dial

When we need more peers, we pick them randomly from the addrbook with some
configurable bias for unvetted peers. The bias should be lower when we have fewer peers
and can increase as we obtain more, ensuring that our first peers are more trustworthy,
but always giving us the chance to discover new good peers.

We track the last time we dialed a peer and the number of unsuccessful attempts
we've made. If too many attempts are made, we mark the peer as bad.

Connection attempts are made with exponential backoff (plus jitter). Because
the selection process happens every `ensurePeersPeriod`, we might not end up
dialing a peer for much longer than the backoff duration.

## Select Peers to Exchange

When we’re asked for peers, we select them as follows:
- select at most `maxGetSelection` peers
- try to select at least `minGetSelection` peers - if we have less than that, select them all.
- select a random, unbiased `getSelectionPercent` of the peers

Send the selected peers. Note we select peers for sending without bias for vetted/unvetted.

## Preventing Spam

There are various cases where we decide a peer has misbehaved and we disconnect from them.
When this happens, the peer is removed from the address book and black listed for
some amount of time. We call this "Disconnect and Mark".
Note that the bad behaviour may be detected outside the PEX reactor itself
(for instance, in the mconnection, or another reactor), but it must be communicated to the PEX reactor
so it can remove and mark the peer.

In the PEX, if a peer sends us unsolicited lists of peers,
or if the peer sends too many requests for more peers in a given amount of time,
we Disconnect and Mark.

## Trust Metric

The quality of peers can be tracked in more fine-grained detail using a
Proportional-Integral-Derivative (PID) controller that incorporates
current, past, and rate-of-change data to inform peer quality.

While a PID trust metric has been implemented, it remains for future work
to use it in the PEX.

See the [trustmetric](../../../architecture/adr-006-trust-metric.md )
and [trustmetric useage](../../../architecture/adr-007-trust-metric-usage.md )
architecture docs for more details.

