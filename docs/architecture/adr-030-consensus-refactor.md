# ADR 030: Consensus Refactor

## Context

One of the biggest challenges this project faces is to proof that the
implementations of the specifications are correct, much like we strive to
formaly verify our alogrithms and protocols we should work towards high
confidence about the correctness of our program code. One of those is the core
of Tendermint - Consensus - which currently resides in the `consensus` package.
Over time there has been high friction making changes to the package due to the
algorithm being scattered in a side-effectful container (the current
`ConsensusState`). In order to test the algorithm a large object-graph needs to
be set up and even than the non-deterministic parts of the container makes will
prevent high certainty. Where ideally we have a 1-to-1 representation of the
[spec](https://github.com/tendermint/spec), ready and easy to test for domain
experts.

Addresses:

- [#1495](https://github.com/tendermint/tendermint/issues/1495)
- [#1692](https://github.com/tendermint/tendermint/issues/1692)

## Decision

To remedy these issues we plan a gradual, non-invasive refactoring of the
`consensus` package. Starting of by isolating the consensus alogrithm into
a pure function and a finite state machine to address the most pressuring issue
of lack of confidence. Doing so while leaving the rest of the package in tact
and have follow-up optional changes to improve the sepration of concerns.

### Implementation changes

The core of Consensus can be modelled as a function with clear defined inputs:

* `State` - data container for current round, height, etc.
* `Event`- significant events in the network

producing clear outputs;

* `State` - updated input
* `Message` - signal what actions to perform

```go
type Event int

const (
	EventUnknown Event = iota
	EventProposal
	Majority23PrevotesBlock
	Majority23PrecommitBlock
	Majority23PrevotesAny
	Majority23PrecommitAny
	TimeoutNewRound
	TimeoutPropose
	TimeoutPrevotes
	TimeoutPrecommit
)

type Message int

const (
	MeesageUnknown Message = iota
	MessageProposal
	MessageVotes
	MessageDecision
)

type State struct {
	height      uint64
	round       uint64
	step        uint64
	lockedValue interface{} // TODO: Define proper type.
	lockedRound interface{} // TODO: Define proper type.
	validValue  interface{} // TODO: Define proper type.
	validRound  interface{} // TODO: Define proper type.
	// From the original notes: valid(v)
	valid       interface{} // TODO: Define proper type.
	// From the original notes: proposer(h, r)
	proposer    interface{} // TODO: Define proper type.
}

func Consensus(Event, State) (State, Message) {
	// Consolidate implementation.
}
```

Tracking of relevant information to feed `Event` into the function and act on
the output is left to the `ConsensusExecutor` (formerly `ConsensusState`). 

Benefits for testing surfacing nicely as testing for a sequence of events
against algorithm could be as simple as the following example:

``` go
func TestConsensusXXX(t *testing.T) {
	type expected struct {
		message Message
		state   State
	}

	// Setup order of events, initial state and expectation.
	var (
		events = []struct {
			event Event
			want  expected
		}{
		// ...
		}
		state = State{
		// ...
		}
	)

	for _, e := range events {
		sate, msg = Consensus(e.event, state)

		// Test message expectation.
		if msg != e.want.message {
			t.Fatalf("have %v, want %v", msg, e.want.message)
		}

		// Test state expectation.
		if !reflect.DeepEqual(state, e.want.state) {
			t.Fatalf("have %v, want %v", state, e.want.state)
		}
	}
}
```

### Implementation roadmap

* implement proposed implementation
* replace currently scattered calls in `ConsensusState` with calls to the new
  `Consensus` function
* rename `ConsensusState` to `ConsensusExecutor` to avoid confusion
* propose design for improved separation and clear information flow between
  `ConsensusExecutor` and `ConsensusReactor`

## Status

Draft.

## Consequences

### Positive

- isolated implementation of the algorithm
- improved testability - simpler to proof correctness
- clearer separation of concerns - easier to reason

### Negative

### Neutral
