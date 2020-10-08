# Maverick

![](https://assets.rollingstone.com/assets/2015/article/tom-cruise-to-fight-drones-in-top-gun-sequel-20150629/201166/large_rect/1435581755/1401x788-Top-Gun-3.jpg)

A byzantine node used to test the tendermint consensus against a plethora of different faulty behaviour. Designed to easily create new malificent behaviour to examine how a tendermint network reacts to the behaviour. Good for fuzzy testing with different network arrangements.

## Behaviors

Each behavior must comply to the following interface (consensus/behavior.go)

```go
type Behavior interface {
	String() string

	EnterPropose(cs *State, height int64, round int32)

	EnterPrevote(cs *State, height int64, round int32)

	EnterPrecommit(cs *State, height int64, round int32)

	ReceivePrevote(cs *State, prevote *types.Vote)

	ReceivePrecommit(cs *State, precommit *types.Vote)

	ReceiveProposal(cs *State, proposal *types.Proposal) error
}
```

At each of these events, the node can exhibit a different behavior. If the normal behavior is desired then all you need to do is to insert the corresponding default function (see `DefaultBehavior`). After creating these behaviors register them in the behaviors.go file (different file to the once in the consensus directory)

## Setup

The maverick node takes most of the functionality from the existing tendermint cli. To install this, in the directory of this readme, run:

```
go install
```

Use `maverick init` to initialize a single node and `maverick node` to run it. There are two other additional flags that you can use for a single node:

-   `behavior` e.g. `--behavior=EquivocationBehavior` to individually a select a behavior for this node. By default the maverick node will toggle between all registered behaviors

-   `height` e.g. `--height=3` to dictate how often the node should execute one of the behaviors. Default is 1 which means that each height, a node will rotate to a new behavior. If the interval is 3, this means that for two heights the node will behave normally and on the third height will rotate to one of the predefined heights.
