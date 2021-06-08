/*
Package blockchain provides two implementations of the fast-sync protocol.

- v0 was the very first implementation. it's battle tested, but does not have a
lot of test coverage.
- v2 is the newest implementation, with a focus on testability and readability.

Check out ADR-40 for the formal model and requirements.

# Termination criteria

1. the maximum peer height is reached
2. termination timeout is triggered, which is set if the peer set is empty or
there are no pending requests.

*/
package blockchain
