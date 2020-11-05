# Jepsen Tests

[Jepsen](https://github.com/jepsen-io/jepsen) is a framework for distributed
systems verification, with fault injection, written in Clojure.

For more information, visit their [website](https://jepsen.io/).

## Test scenarios

Jepsen tests should give us some assurance that Tendermint produces
linearizable history in the presence of:

* Network partitions
* Clock skews
* Crashes
* Changing validators
* Truncating logs

NOTE: e2e tests check Tendermint recovers after crashes, but they do not check
the transaction history. Jepsen test suite can be viewed as an extension in
this way.

## Running
