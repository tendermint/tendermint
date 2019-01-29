# Simple `kvstore` Test

This test scenario is a very simple validation of whether or not the `kvstore`
proxy application is running correctly on a particular Tendermint network.

Steps to validation:

1. Generate a random hexadecimal string
2. Put the string into the `kvstore` under the `test_value` key through one node
3. Retrieve the value under the `test_value` key from a different node
4. Check that the retrieved value corresponds to the original value
