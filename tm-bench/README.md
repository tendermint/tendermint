# Tendermint blockchain benchmarking tool (tm-bench)

`tm-bench` is a simple benchmarking tool for [Tendermint
core](https://github.com/tendermint/tendermint) nodes.

```
λ tm-bench -T 10 -r 1000 localhost:46657
Stats             Avg        Stdev      Max
Block latency     6.18ms     3.19ms     14ms
Blocks/sec        0.828      0.378      1
Txs/sec           963        493        1811
```

* [QuickStart using Docker](#quickstart-using-docker)
* [QuickStart using binaries](#quickstart-using-binaries)
* [Usage](#usage)

## QuickStart using Docker

```
docker run -it --rm -v "/tmp:/tendermint" tendermint/tendermint init
docker run -it --rm -v "/tmp:/tendermint" -p "46657:46657" --name=tm tendermint/tendermint

docker run -it --rm --link=tm tendermint/bench tm:46657
```

## QuickStart using binaries

Linux:

```
curl -L https://s3-us-west-2.amazonaws.com/tendermint/0.8.0/tendermint_linux_amd64.zip && sudo unzip -d /usr/local/bin tendermint_linux_amd64.zip && sudo chmod +x tendermint
tendermint init
tendermint node --app_proxy=dummy

tm-bench localhost:46657
```

Max OS:

```
curl -L https://s3-us-west-2.amazonaws.com/tendermint/0.8.0/tendermint_darwin_amd64.zip && sudo unzip -d /usr/local/bin tendermint_darwin_amd64.zip && sudo chmod +x tendermint
tendermint init
tendermint node --app_proxy=dummy

tm-bench localhost:46657
```

## Usage

```
tm-bench [-c 1] [-T 10] [-r 1000] [endpoints]

Examples:
        tm-bench localhost:46657
Flags:
  -T int
        Exit after the specified amount of time in seconds (default 10)
  -c int
        Connections to keep open per endpoint (default 1)
  -r int
        Txs per second to send in a connection (default 1000)
  -v    Verbose output
```
