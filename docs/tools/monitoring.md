# tm-monitor

Tendermint blockchain monitoring tool; watches over one or more nodes,
collecting and providing various statistics to the user:

- [https://github.com/tendermint/tendermint/tree/master/tools/tm-monitor](https://github.com/tendermint/tendermint/tree/master/tools/tm-monitor)

## Quick Start

### Docker

Assuming your application is running in another container with the name
`app`:

```
docker run -it --rm -v "/tmp:/tendermint" tendermint/tendermint init
docker run -it --rm -v "/tmp:/tendermint" -p "26657:26657" --name=tm --link=app tendermint/tendermint node --proxy_app=tcp://app:26658

docker run -it --rm -p "26670:26670" --link=tm tendermint/monitor tm:26657
```

If you don't have an application yet, but still want to try monitor out,
use `kvstore`:

```
docker run -it --rm -v "/tmp:/tendermint" tendermint/tendermint init
docker run -it --rm -v "/tmp:/tendermint" -p "26657:26657" --name=tm tendermint/tendermint node --proxy_app=kvstore
```

```
docker run -it --rm -p "26670:26670" --link=tm tendermint/monitor tm:26657
```

### Using Binaries

[Install Tendermint](../introduction/install.md).

Start a Tendermint node:

```
tendermint init
tendermint node --proxy_app=kvstore
```

In another window, run the monitor:

```
tm-monitor localhost:26657
```

## Usage

```
tm-monitor [-v] [-no-ton] [-listen-addr="tcp://0.0.0.0:26670"] [endpoints]

Examples:
        # monitor single instance
        tm-monitor localhost:26657

        # monitor a few instances by providing comma-separated list of RPC endpoints
        tm-monitor host1:26657,host2:26657
Flags:
  -listen-addr string
        HTTP and Websocket server listen address (default "tcp://0.0.0.0:26670")
  -no-ton
        Do not show ton (table of nodes)
  -v    verbose logging
```

### RPC UI

Run `tm-monitor` and visit http://localhost:26670 You should see the
list of the available RPC endpoints:

```
http://localhost:26670/status
http://localhost:26670/status/network
http://localhost:26670/monitor?endpoint=_
http://localhost:26670/status/node?name=_
http://localhost:26670/unmonitor?endpoint=_
```

The API is available as GET requests with URI encoded parameters, or as
JSONRPC POST requests. The JSONRPC methods are also exposed over
websocket.

## Development

```
make tools
make test
```
