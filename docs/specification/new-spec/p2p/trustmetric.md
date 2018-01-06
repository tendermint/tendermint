
The trust metric tracks the quality of the peers.
When a peer exceeds a certain quality for a certain amount of time,
it is marked as vetted in the addrbook.
If a vetted peer's quality degrades sufficiently, it is booted, and must prove itself from scratch.
If we need to make room for a new vetted peer, we move the lowest scoring vetted peer back to unvetted.
If we need to make room for a new unvetted peer, we remove the lowest scoring unvetted peer -
possibly only if its below some absolute minimum ?

Peer quality is tracked in the connection and across the reactors.
Behaviours are defined as one of:
    - fatal - something outright malicious. we should disconnect and remember them.
    - bad - any kind of timeout, msgs that dont unmarshal, or fail other validity checks, or msgs we didn't ask for or arent expecting
    - neutral - normal correct behaviour. unknown channels/msg types (version upgrades).
    - good - some random majority of peers per reactor sending us useful messages

