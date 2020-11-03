# jepsen.tendermint

Jepsen tests for the Tendermint distributed consensus system.

## Building

- Clone this repo
- Install JDK8 or higher
- Install [Leiningen](https://leiningen.org/)
- Optional: install gnuplot
- In the tendermint repository, run `lein run test`

To build a fat JAR file that you can run independently of leiningen, run `lein
uberjar`.

## Usage

`lein run serve` runs a web server for browsing test results.

`lein run test` runs a test. Use `lein run test --help` to see options: you'll
likely want to set `--node some.hostname ...` or `--nodes-file some_file`, and
adjust `--username` as desired.

For instance, to test a shifting cluster membership, with two clients per node,
for 3000 seconds, over a group of compare-and-set registers, you'd run

```
lein run test --nemesis changing-validators --concurrency 2n --time-limit 3000 --workload cas-register
```

Or to show that a duplicated validator with just shy of 2/3 of the vote can split the blockchain during a partition, leading to the loss of acknowledged writes:

```
lein run test --node n1 --node n2 --node n3 --node n4 --node n5 --node n6 --node n7 --dup-validators --super-byzantine-validators --nemesis split-dup-validators --workload set --concurrency 5n --test-count 15 --time-limit 30
```

## License

Copyright © 2017 Jepsen, LLC

Distributed under the Apache Public License 2.0.
