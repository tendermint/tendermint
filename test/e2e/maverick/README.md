# Maverick

![](https://assets.rollingstone.com/assets/2015/article/tom-cruise-to-fight-drones-in-top-gun-sequel-20150629/201166/large_rect/1435581755/1401x788-Top-Gun-3.jpg)

A byzantine node used to test Tendermint consensus against a plethora of different faulty misbehaviors. Designed to easily create new faulty misbehaviors to examine how a Tendermint network reacts to the misbehavior. Can also be used for fuzzy testing with different network arrangements.

## Misbehaviors

A misbehavior allows control at the following stages as highlighted by the struct below

```go
type Misbehavior struct {
	String string

	EnterPropose func(cs *State, height int64, round int32)

	EnterPrevote func(cs *State, height int64, round int32)

	EnterPrecommit func(cs *State, height int64, round int32)

	ReceivePrevote func(cs *State, prevote *types.Vote)

	ReceivePrecommit func(cs *State, precommit *types.Vote)

	ReceiveProposal func(cs *State, proposal *types.Proposal) error
}
```

At each of these events, the node can exhibit a different misbehavior. To create a new misbehavior define a function that builds off the existing default misbehavior and then overrides one or more of these functions. Then append it to the misbehaviors list so the node recognizes it like so:

```go
var MisbehaviorList = map[string]Misbehavior{
	"double-prevote": DoublePrevoteMisbehavior(),
}
```

## Setup

The maverick node takes most of the functionality from the existing Tendermint CLI. To install this, in the directory of this readme, run:

```bash
go build
```

Use `maverick init` to initialize a single node and `maverick node` to run it. This will run it normally unless you use the misbehaviors flag as follows:

```bash
maverick node --proxy_app persistent_kvstore --misbehaviors double-vote,10
```

This would cause the node to vote twice in every round at height 10. To add more misbehaviors at different heights, append the next misbehavior and height after the first (with comma separation).
