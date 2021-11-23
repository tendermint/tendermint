# tendermint-test
Testing tendermint using the framework [Scheduler Test Server](https://github.com/ds-test-framework/scheduler/blob/master/docs/testserver.md)

## Organization
- [`util`](./util) is used to parse the byte array of message and convert it to `tendermint` message and to change `Vote` signatures using replica private keys
- [`testcases`](./testcases) describe the test scenarios
- [`server.go`](./server.go) instantiates a testing server and runs it.

## Development
- Update the `ServerConfig` in `server.go` for the right address to run the server on and run `go run server.go` to start the testing process.
- This repository does not contain the changes needed on the tendermint codebase to ensure the replicas communicate with the test server
- The replicas need to be run separately and configured to talk to the test server

## Scenarios being tested

1. Round skipping: We ensure that the replicas don't achieve consensus in a single round and move on to the next.
- [Using Prevotes](testcases/rskip/one.go)

    We partition the set of `n=3f+1` replicas into parts of size `1, f and 2f` with labels say `h, faulty and rest`. In round `0` 
    1. We delay the `PreVote` messages of `h` to `rest` and `faulty`
    2. We change the `PreVote` messages originating from `faulty` to `nil` votes and deliver them.

    This ensures that all replicas except `h` does not see consensus and will `PreCommit` nil which will trigger a round change
2. Locked value: We know that once a replica sees `2f+1 PreVotes` it locks on the proposed block, we simulate scenarios where the locked block can be changed or unlocked
- [Lock, unlock and verify new prevote, Lock, don't send proposal and check prevote](testcases/lockedvalue/one.go)

    We partition the set of `n=3f+1` replicas into `(h,1), (faulty, f) and (rest, 2f)` similar to the earlier scenario.

    In round `0`:
    1. Record the `Proposal` as `oldProposal`
    2. We delay the `PreVote` message of `h` to `rest` and `faulty`
    3. We change the `PreVote` messages originating from the `faulty` set such that they vote for `nil`

    We will see a round skip because enough replicas don't see consensus. In round `1`: 
    1. We delay the delivery of `Proposal` messages to all replicas and check if `h` votes for `oldProposal`, fail if not
    
    Since only `h` would have locked and the rest don't receive the `Proposal`, all replicas vote `nil` and `h` should unlock followed by a round change. In round `2`:
    1. We record the `Proposal` as `newProposal` and verify if `h` votes on `newProposal`

    The two scenarios are combined in a single testcase file [here](testcases/lockedvalue/one.go)

- [Lock, send different proposal and test prevote](testcases/sanity/two.go)

    We select `f` faulty replicas from the set of `n=3f+1` replicas. In round 0:
    1. We change the `PreVote` and `PreCommit` messages of faulty replicas to nil
    2. We deliver `2f` correct `PreCommit` votes and `1` `nil` vote to all replicas and hence force round skip

    In round `1`:
    1. We change the `Proposal` message to propose `nil` block and wait for replicas to commit the proposal from round `0`, The testcase fails if the replicas move to `round2`

