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
docker run -it --rm -v "/tmp:/tendermint" -p "46657:46657" tendermint/tendermint

docker run -it --rm tendermint/tm-monitor
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
# monitor single instance
tm-monitor localhost:46657

# monitor a few instances by providing comma-separated list of RPC endpoints
tm-monitor host1:46657,host2:46657
```

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

1. Currently we get IPs and dial, but should reverse so the nodes dial the netmon, both for node privacy and easier reconfig (validators changing ip/port).
2. Uptime over last day, month, year (Q: how do I get this?)
3. `statsd` metrics
4. log metrics for charts (Q: what charts?)
5. show network size (Q: how do I get the number?)
6. metrics RPC (Q: do we need this?)
