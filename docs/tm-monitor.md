# Tendermint monitor (tm-monitor)

Tendermint monitor watches over one or more [Tendermint
core](https://github.com/tendermint/tendermint) applications (nodes),
collecting and providing various statistics to the user.

* [QuickStart using Docker](#quickstart-using-docker)
* [QuickStart using binaries](#quickstart-using-binaries)
* [Usage](#usage)
* [RPC UI](#rpc-ui)

## QuickStart using Docker

```
docker run -it --rm -v "/tmp:/tendermint" tendermint/tendermint init
docker run -it --rm -v "/tmp:/tendermint" -p "46657:46657" --name=tm tendermint/tendermint

docker run -it --rm --link=tm tendermint/monitor tm:46657
```

## QuickStart using binaries

Linux:

```
curl -L https://s3-us-west-2.amazonaws.com/tendermint/0.8.0/tendermint_linux_amd64.zip && sudo unzip -d /usr/local/bin tendermint_linux_amd64.zip && sudo chmod +x tendermint
tendermint init
tendermint node --app_proxy=dummy

tm-monitor localhost:46657
```

Max OS:

```
curl -L https://s3-us-west-2.amazonaws.com/tendermint/0.8.0/tendermint_darwin_amd64.zip && sudo unzip -d /usr/local/bin tendermint_darwin_amd64.zip && sudo chmod +x tendermint
tendermint init
tendermint node --app_proxy=dummy

tm-monitor localhost:46657
```

## Usage

```
tm-monitor [-v] [-no-ton] [-listen-addr="tcp://0.0.0.0:46670"] [endpoints]

Examples:
        # monitor single instance
        tm-monitor localhost:46657

        # monitor a few instances by providing comma-separated list of RPC endpoints
        tm-monitor host1:46657,host2:46657
Flags:
  -listen-addr string
        HTTP and Websocket server listen address (default "tcp://0.0.0.0:46670")
  -no-ton
        Do not show ton (table of nodes)
  -v    verbose logging
```

[![asciicast](https://asciinema.org/a/105974.png)](https://asciinema.org/a/105974)

### RPC UI

Run `tm-monitor` and visit [http://localhost:46670](http://localhost:46670).
You should see the list of the available RPC endpoints:

```
http://localhost:46670/status
http://localhost:46670/status/network
http://localhost:46670/monitor?endpoint=_
http://localhost:46670/status/node?name=_
http://localhost:46670/unmonitor?endpoint=_
```

The API is available as GET requests with URI encoded parameters, or as JSONRPC
POST requests. The JSONRPC methods are also exposed over websocket.

### Ideas

- currently we get IPs and dial, but should reverse so the nodes dial the
  netmon, both for node privacy and easier reconfig (validators changing
  ip/port). It would be good to have both. For testnets with others we def need
  them to dial the monitor. But I want to be able to run the monitor from my
  laptop without openning ports.
  If we don't want to open all the ports, maybe something like this would be a
  good fit for us: tm-monitor agent running on each node, collecting all the
  metrics. Each tm-monitor agent monitors local TM node and sends stats to a
  single master tm-monitor master. That way we'll only need to open a single
  port for UI on the node with tm-monitor master. And I believe it could be
  done with a single package with a few subcommands.
  ```
  # agent collecting metrics from localhost (default)
  tm-monitor agent --master="192.168.1.17:8888"

  # agent collecting metrics from another TM node (useful for testing, development)
  tm-monitor agent --master="192.168.1.17:8888" --node="192.168.1.18:46657"

  # master accepting stats from agents
  tm-monitor master [--ton] OR [--ui] (`--ui` mode by default)

  # display table of nodes in the terminal (useful for testing, development, playing with TM)
  # --nodes="localhost:46657" by default
  tm-monitor

  # display table of nodes in the terminal (useful for testing, development, playing with TM)
  tm-monitor --nodes="192.168.1.18:46657,192.168.1.19:46657"
  ```
- uptime over last day, month, year. There are different meanings for uptime.
  One is to constantly ping the nodes and make sure they respond to eg.
  /status. A more fine-grained one is to check for votes in the block commits.
- show network size + auto discovery. You can get a list of connected peers at
  /net_info. But no single one will be connected to the whole network, so need
  to tease out all the unique peers from calling /net_info on all of them.
  Unless you have some prior information about how many peers in the net ...
  More: we could add `-auto-discovery` option and try to connect to every node.
- input plugin for https://github.com/influxdata/telegraf, so the user is able
  to get the metrics and send them whenever he wants to (grafana, prometheus,
  etc.).

Feel free to vote on the ideas or add your own by saying hello on
[Slack](http://forum.tendermint.com:3000/) or by opening an issue.
