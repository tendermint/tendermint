---
order: 4
---

# Running in production

If you are building Tendermint from source for use in production, make sure to check out an appropriate Git tag instead of a branch.

## Database

By default, Tendermint uses the `syndtr/goleveldb` package for its in-process
key-value database. If you want maximal performance, it may be best to install
the real C-implementation of LevelDB and compile Tendermint to use that using
`make build TENDERMINT_BUILD_OPTIONS=cleveldb`. See the [install
instructions](../introduction/install.md) for details.

Tendermint keeps multiple distinct databases in the `$TMROOT/data`:

- `blockstore.db`: Keeps the entire blockchain - stores blocks,
  block commits, and block meta data, each indexed by height. Used to sync new
  peers.
- `evidence.db`: Stores all verified evidence of misbehaviour.
- `state.db`: Stores the current blockchain state (ie. height, validators,
  consensus params). Only grows if consensus params or validators change. Also
  used to temporarily store intermediate results during block processing.
- `tx_index.db`: Indexes txs (and their results) by tx hash and by DeliverTx result events.

By default, Tendermint will only index txs by their hash and height, not by their DeliverTx
result events. See [indexing transactions](../app-dev/indexing-transactions.md) for
details.

Applications can expose block pruning strategies to the node operator. Please read the documentation of your application
to find out more details.

Applications can use [state sync](state-sync.md) to help nodes bootstrap quickly.

## Logging

Default logging level (`log-level = "info"`) should suffice for
normal operation mode. Read [this
post](https://blog.cosmos.network/one-of-the-exciting-new-features-in-0-10-0-release-is-smart-log-level-flag-e2506b4ab756)
for details on how to configure `log-level` config variable. Some of the
modules can be found [here](logging.md#list-of-modules). If
you're trying to debug Tendermint or asked to provide logs with debug
logging level, you can do so by running Tendermint with
`--log-level="debug"`.

### Consensus WAL

Tendermint uses a write ahead log (WAL) for consensus. The `consensus.wal` is used to ensure we can recover from a crash at any point
in the consensus state machine. It writes all consensus messages (timeouts, proposals, block part, or vote)
to a single file, flushing to disk before processing messages from its own
validator. Since Tendermint validators are expected to never sign a conflicting vote, the
WAL ensures we can always recover deterministically to the latest state of the consensus without
using the network or re-signing any consensus messages. The consensus WAL max size of 1GB and is automatically rotated.

If your `consensus.wal` is corrupted, see [below](#wal-corruption).

## DOS Exposure and Mitigation

Validators are supposed to setup [Sentry Node
Architecture](./validators.md)
to prevent Denial-of-service attacks.

### P2P

The core of the Tendermint peer-to-peer system is `MConnection`. Each
connection has `MaxPacketMsgPayloadSize`, which is the maximum packet
size and bounded send & receive queues. One can impose restrictions on
send & receive rate per connection (`SendRate`, `RecvRate`).

The number of open P2P connections can become quite large, and hit the operating system's open
file limit (since TCP connections are considered files on UNIX-based systems). Nodes should be
given a sizable open file limit, e.g. 8192, via `ulimit -n 8192` or other deployment-specific
mechanisms.

### RPC

Endpoints returning multiple entries are limited by default to return 30
elements (100 max). See the [RPC Documentation](https://docs.tendermint.com/master/rpc/)
for more information.

Rate-limiting and authentication are another key aspects to help protect
against DOS attacks. Validators are supposed to use external tools like
[NGINX](https://www.nginx.com/blog/rate-limiting-nginx/) or
[traefik](https://doc.traefik.io/traefik/middlewares/http/ratelimit/)
to achieve the same things.

## Debugging Tendermint

If you ever have to debug Tendermint, the first thing you should probably do is
check out the logs. See [Logging](../nodes/logging.md), where we
explain what certain log statements mean.

If, after skimming through the logs, things are not clear still, the next thing
to try is querying the `/status` RPC endpoint. It provides the necessary info:
whenever the node is syncing or not, what height it is on, etc.

```bash
curl http(s)://{ip}:{rpcPort}/status
```

`/dump_consensus_state` will give you a detailed overview of the consensus
state (proposer, latest validators, peers states). From it, you should be able
to figure out why, for example, the network had halted.

```bash
curl http(s)://{ip}:{rpcPort}/dump_consensus_state
```

There is a reduced version of this endpoint - `/consensus_state`, which returns
just the votes seen at the current height.

If, after consulting with the logs and above endpoints, you still have no idea
what's happening, consider using `tendermint debug kill` sub-command. This
command will scrap all the available info and kill the process. See
[Debugging](../tools/debugging/README.md) for the exact format.

You can inspect the resulting archive yourself or create an issue on
[Github](https://github.com/tendermint/tendermint). Before opening an issue
however, be sure to check if there's [no existing
issue](https://github.com/tendermint/tendermint/issues) already.

## Monitoring Tendermint

Each Tendermint instance has a standard `/health` RPC endpoint, which responds
with 200 (OK) if everything is fine and 500 (or no response) - if something is
wrong.

Other useful endpoints include mentioned earlier `/status`, `/net_info` and
`/validators`.

Tendermint also can report and serve Prometheus metrics. See
[Metrics](./metrics.md).

`tendermint debug dump` sub-command can be used to periodically dump useful
information into an archive. See [Debugging](../tools/debugging/README.md) for more
information.

## What happens when my app dies

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
signals we use the default behavior in Go: [Default behavior of signals
in Go
programs](https://golang.org/pkg/os/signal/#hdr-Default_behavior_of_signals_in_Go_programs).

## Corruption

**NOTE:** Make sure you have a backup of the Tendermint data directory.

### Possible causes

Remember that most corruption is caused by hardware issues:

- RAID controllers with faulty / worn out battery backup, and an unexpected power loss
- Hard disk drives with write-back cache enabled, and an unexpected power loss
- Cheap SSDs with insufficient power-loss protection, and an unexpected power-loss
- Defective RAM
- Defective or overheating CPU(s)

Other causes can be:

- Database systems configured with fsync=off and an OS crash or power loss
- Filesystems configured to use write barriers plus a storage layer that ignores write barriers. LVM is a particular culprit.
- Tendermint bugs
- Operating system bugs
- Admin error (e.g., directly modifying Tendermint data-directory contents)

(Source: <https://wiki.postgresql.org/wiki/Corruption>)

### WAL Corruption

If consensus WAL is corrupted at the latest height and you are trying to start
Tendermint, replay will fail with panic.

Recovering from data corruption can be hard and time-consuming. Here are two approaches you can take:

1. Delete the WAL file and restart Tendermint. It will attempt to sync with other peers.
2. Try to repair the WAL file manually:

1) Create a backup of the corrupted WAL file:

    ```sh
    cp "$TMHOME/data/cs.wal/wal" > /tmp/corrupted_wal_backup
    ```

2) Use `./scripts/wal2json` to create a human-readable version:

    ```sh
    ./scripts/wal2json/wal2json "$TMHOME/data/cs.wal/wal" > /tmp/corrupted_wal
    ```

3) Search for a "CORRUPTED MESSAGE" line.
4) By looking at the previous message and the message after the corrupted one
   and looking at the logs, try to rebuild the message. If the consequent
   messages are marked as corrupted too (this may happen if length header
   got corrupted or some writes did not make it to the WAL ~ truncation),
   then remove all the lines starting from the corrupted one and restart
   Tendermint.

    ```sh
    $EDITOR /tmp/corrupted_wal
    ```

5) After editing, convert this file back into binary form by running:

    ```sh
    ./scripts/json2wal/json2wal /tmp/corrupted_wal  $TMHOME/data/cs.wal/wal
    ```

## Hardware

### Processor and Memory

While actual specs vary depending on the load and validators count, minimal
requirements are:

- 1GB RAM
- 25GB of disk space
- 1.4 GHz CPU

SSD disks are preferable for applications with high transaction throughput.

Recommended:

- 2GB RAM
- 100GB SSD
- x64 2.0 GHz 2v CPU

While for now, Tendermint stores all the history and it may require significant
disk space over time, we are planning to implement state syncing (See [this
issue](https://github.com/tendermint/tendermint/issues/828)). So, storing all
the past blocks will not be necessary.

### Validator signing on 32 bit architectures (or ARM)

Both our `ed25519` and `secp256k1` implementations require constant time
`uint64` multiplication. Non-constant time crypto can (and has) leaked
private keys on both `ed25519` and `secp256k1`. This doesn't exist in hardware
on 32 bit x86 platforms ([source](https://bearssl.org/ctmul.html)), and it
depends on the compiler to enforce that it is constant time. It's unclear at
this point whenever the Golang compiler does this correctly for all
implementations.

**We do not support nor recommend running a validator on 32 bit architectures OR
the "VIA Nano 2000 Series", and the architectures in the ARM section rated
"S-".**

### Operating Systems

Tendermint can be compiled for a wide range of operating systems thanks to Go
language (the list of \$OS/\$ARCH pairs can be found
[here](https://golang.org/doc/install/source#environment)).

While we do not favor any operation system, more secure and stable Linux server
distributions (like Centos) should be preferred over desktop operation systems
(like Mac OS).

Native Windows support is not provided. If you are using a windows machine, you can try using the [bash shell](https://docs.microsoft.com/en-us/windows/wsl/install-win10).

### Miscellaneous

NOTE: if you are going to use Tendermint in a public domain, make sure
you read [hardware recommendations](https://cosmos.network/validators) for a validator in the
Cosmos network.

## Configuration parameters

- `p2p.flush-throttle-timeout`
- `p2p.max-packet-msg-payload-size`
- `p2p.send-rate`
- `p2p.recv-rate`

If you are going to use Tendermint in a private domain and you have a
private high-speed network among your peers, it makes sense to lower
flush throttle timeout and increase other params.

```toml
[p2p]
send-rate=20000000 # 2MB/s
recv-rate=20000000 # 2MB/s
flush-throttle-timeout=10
max-packet-msg-payload-size=10240 # 10KB
```

- `mempool.broadcast`

Setting this to false will stop the mempool from relaying transactions
to other peers until they are included in a block. It means only the
peer you send the tx to will see it until it is included in a block.

- `consensus.skip-timeout-commit`

We want `skip-timeout-commit=false` when there is economics on the line
because proposers should wait to hear for more votes. But if you don't
care about that and want the fastest consensus, you can skip it. It will
be kept false by default for public deployments (e.g. [Cosmos
Hub](https://hub.cosmos.network/main/hub-overview/overview.html)) while for enterprise
applications, setting it to true is not a problem.

- `consensus.peer-gossip-sleep-duration`

You can try to reduce the time your node sleeps before checking if
theres something to send its peers.

- `consensus.timeout-commit`

You can also try lowering `timeout-commit` (time we sleep before
proposing the next block).

- `p2p.addr-book-strict`

By default, Tendermint checks whenever a peer's address is routable before
saving it to the address book. The address is considered as routable if the IP
is [valid and within allowed
ranges](https://github.com/tendermint/tendermint/blob/27bd1deabe4ba6a2d9b463b8f3e3f1e31b993e61/p2p/netaddress.go#L209).

This may not be the case for private or local networks, where your IP range is usually
strictly limited and private. If that case, you need to set `addr-book-strict`
to `false` (turn it off).

- `rpc.max-open-connections`

By default, the number of simultaneous connections is limited because most OS
give you limited number of file descriptors.

If you want to accept greater number of connections, you will need to increase
these limits.

[Sysctls to tune the system to be able to open more connections](https://github.com/satori-com/tcpkali/blob/master/doc/tcpkali.man.md#sysctls-to-tune-the-system-to-be-able-to-open-more-connections)

The process file limits must also be increased, e.g. via `ulimit -n 8192`.

...for N connections, such as 50k:

```md
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
`rpc.grpc-max-open-connections`.
