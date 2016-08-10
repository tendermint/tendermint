# The core consensus algorithm.

* state.go - The state machine as detailed in the whitepaper
* reactor.go - A reactor that connects the state machine to the gossip network

# Go-routine summary

The reactor runs 2 go-routines for each added peer: gossipDataRoutine and gossipVotesRoutine.

The consensus state runs two persistent go-routines: timeoutRoutine and receiveRoutine.
Go-routines are also started to trigger timeouts and to avoid blocking when the internalMsgQueue is really backed up.

# Replay/WAL

A write-ahead log is used to record all messages processed by the receiveRoutine,
which amounts to all inputs to the consensus state machine:
messages from peers, messages from ourselves, and timeouts.
They can be played back deterministically at startup or using the replay console. 
