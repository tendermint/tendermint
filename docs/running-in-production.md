# Running in production

## Logging

Default logging level (`main:info,state:info,*:`) should suffice for
normal operation mode. Read [this
post](https://blog.cosmos.network/one-of-the-exciting-new-features-in-0-10-0-release-is-smart-log-level-flag-e2506b4ab756)
for details on how to configure `log_level` config variable. Some of the
modules can be found [here](./how-to-read-logs.md#list-of-modules). If
you're trying to debug Tendermint or asked to provide logs with debug
logging level, you can do so by running tendermint with
`--log_level="*:debug"`.

## DOS Exposure and Mitigation

Validators are supposed to setup [Sentry Node
Architecture](https://blog.cosmos.network/tendermint-explained-bringing-bft-based-pos-to-the-public-blockchain-domain-f22e274a0fdb)
to prevent Denial-of-service attacks. You can read more about it
[here](https://github.com/tendermint/aib-data/blob/develop/medium/TendermintBFT.md).

### P2P

The core of the Tendermint peer-to-peer system is `MConnection`. Each
connection has `MaxPacketMsgPayloadSize`, which is the maximum packet
size and bounded send & receive queues. One can impose restrictions on
send & receive rate per connection (`SendRate`, `RecvRate`).

### RPC

Endpoints returning multiple entries are limited by default to return 30
elements (100 max).

Rate-limiting and authentication are another key aspects to help protect
against DOS attacks. While in the future we may implement these
features, for now, validators are supposed to use external tools like
[NGINX](https://www.nginx.com/blog/rate-limiting-nginx/) or
[traefik](https://docs.traefik.io/configuration/commons/#rate-limiting)
to achieve the same things.

## Debugging Tendermint

If you ever have to debug Tendermint, the first thing you should
probably do is to check out the logs. See ["How to read
logs"](./how-to-read-logs.md), where we explain what certain log
statements mean.

If, after skimming through the logs, things are not clear still, the
second TODO is to query the /status RPC endpoint. It provides the
necessary info: whenever the node is syncing or not, what height it is
on, etc.

    $ curl http(s)://{ip}:{rpcPort}/status

`dump_consensus_state` will give you a detailed overview of the
consensus state (proposer, lastest validators, peers states). From it,
you should be able to figure out why, for example, the network had
halted.

    $ curl http(s)://{ip}:{rpcPort}/dump_consensus_state

There is a reduced version of this endpoint - `consensus_state`, which
returns just the votes seen at the current height.

-   [Github Issues](https://github.com/tendermint/tendermint/issues)
-   [StackOverflow
    questions](https://stackoverflow.com/questions/tagged/tendermint)

## Monitoring Tendermint

Each Tendermint instance has a standard `/health` RPC endpoint, which
responds with 200 (OK) if everything is fine and 500 (or no response) -
if something is wrong.

Other useful endpoints include mentioned earlier `/status`, `/net_info` and
`/validators`.

We have a small tool, called `tm-monitor`, which outputs information from
the endpoints above plus some statistics. The tool can be found
[here](https://github.com/tendermint/tools/tree/master/tm-monitor).

Tendermint also can report and serve Prometheus metrics. See
[Metrics](./metrics.md).

## What happens when my app dies?

You are supposed to run Tendermint under a [process
supervisor](https://en.wikipedia.org/wiki/Process_supervision) (like
systemd or runit). It will ensure Tendermint is always running (despite
possible errors).

Getting back to the original question, if your application dies,
Tendermint will panic. After a process supervisor restarts your
application, Tendermint should be able to reconnect successfully. The
order of restart does not matter for it.

## Signal handling

We catch SIGINT and SIGTERM and try to clean up nicely. For other
signals we use the default behaviour in Go: [Default behavior of signals
in Go
programs](https://golang.org/pkg/os/signal/#hdr-Default_behavior_of_signals_in_Go_programs).

## Hardware

### Processor and Memory

While actual specs vary depending on the load and validators count,
minimal requirements are:

-   1GB RAM
-   25GB of disk space
-   1.4 GHz CPU

SSD disks are preferable for applications with high transaction
throughput.

Recommended:

-   2GB RAM
-   100GB SSD
-   x64 2.0 GHz 2v CPU

While for now, Tendermint stores all the history and it may require
significant disk space over time, we are planning to implement state
syncing (See
[this issue](https://github.com/tendermint/tendermint/issues/828)). So,
storing all the past blocks will not be necessary.

### Operating Systems

Tendermint can be compiled for a wide range of operating systems thanks
to Go language (the list of \$OS/\$ARCH pairs can be found
[here](https://golang.org/doc/install/source#environment)).

While we do not favor any operation system, more secure and stable Linux
server distributions (like Centos) should be preferred over desktop
operation systems (like Mac OS).

### Miscellaneous

NOTE: if you are going to use Tendermint in a public domain, make sure
you read [hardware recommendations (see "4.
Hardware")](https://cosmos.network/validators) for a validator in the
Cosmos network.

## Configuration parameters

-   `p2p.flush_throttle_timeout` `p2p.max_packet_msg_payload_size`
    `p2p.send_rate` `p2p.recv_rate`

If you are going to use Tendermint in a private domain and you have a
private high-speed network among your peers, it makes sense to lower
flush throttle timeout and increase other params.

    [p2p]

    send_rate=20000000 # 2MB/s
    recv_rate=20000000 # 2MB/s
    flush_throttle_timeout=10
    max_packet_msg_payload_size=10240 # 10KB

-   `mempool.recheck`

After every block, Tendermint rechecks every transaction left in the
mempool to see if transactions committed in that block affected the
application state, so some of the transactions left may become invalid.
If that does not apply to your application, you can disable it by
setting `mempool.recheck=false`.

-   `mempool.broadcast`

Setting this to false will stop the mempool from relaying transactions
to other peers until they are included in a block. It means only the
peer you send the tx to will see it until it is included in a block.

-   `consensus.skip_timeout_commit`

We want `skip_timeout_commit=false` when there is economics on the line
because proposers should wait to hear for more votes. But if you don't
care about that and want the fastest consensus, you can skip it. It will
be kept false by default for public deployments (e.g. [Cosmos
Hub](https://cosmos.network/intro/hub)) while for enterprise
applications, setting it to true is not a problem.

-   `consensus.peer_gossip_sleep_duration`

You can try to reduce the time your node sleeps before checking if
theres something to send its peers.

-   `consensus.timeout_commit`

You can also try lowering `timeout_commit` (time we sleep before
proposing the next block).

-   `consensus.max_block_size_txs`

By default, the maximum number of transactions per a block is 10_000.
Feel free to change it to suit your needs.

-   `p2p.addr_book_strict`

By default, Tendermint checks whenever a peer's address is routable before
saving it to the address book. The address is considered as routable if the IP
is [valid and within allowed
ranges](https://github.com/tendermint/tendermint/blob/27bd1deabe4ba6a2d9b463b8f3e3f1e31b993e61/p2p/netaddress.go#L209).

This may not be the case for private networks, where your IP range is usually
strictly limited and private. If that case, you need to set `addr_book_strict`
to `false` (turn off).

- `rpc.max_open_connections`

By default, the number of simultaneous connections is limited because most OS
give you limited number of file descriptors.

If you want to accept greater number of connections, you will need to increase
these limits.

[Sysctls to tune the system to be able to open more connections](https://github.com/satori-com/tcpkali/blob/master/doc/tcpkali.man.md#sysctls-to-tune-the-system-to-be-able-to-open-more-connections)

...for N connections, such as 50k:

```
kern.maxfiles=10000+2*N         # BSD
kern.maxfilesperproc=100+2*N    # BSD
kern.ipc.maxsockets=10000+2*N   # BSD
fs.file-max=10000+2*N           # Linux
net.ipv4.tcp_max_orphans=N      # Linux

# For load-generating clients.
net.ipv4.ip_local_port_range="10000  65535"  # Linux.
net.inet.ip.portrange.first=10000  # BSD/Mac.
net.inet.ip.portrange.last=65535   # (Enough for N < 55535)
net.ipv4.tcp_tw_reuse=1         # Linux
net.inet.tcp.maxtcptw=2*N       # BSD

# If using netfilter on Linux:
net.netfilter.nf_conntrack_max=N
echo $((N/8)) > /sys/module/nf_conntrack/parameters/hashsize
```

The similar option exists for limiting the number of gRPC connections -
`rpc.grpc_max_open_connections`.
