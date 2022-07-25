<!---
order: 1
--->

# Creating an application in Go

## Guide Assumptions

This guide is designed for beginners who want to get started with a Tendermint
Core application from scratch. It does not assume that you have any prior
experience with Tendermint Core.

Tendermint Core is Byzantine Fault Tolerant (BFT) middleware that takes a state
transition machine - written in any programming language - and securely
replicates it on many machines.

Although Tendermint Core is written in the Golang programming language, prior
knowledge of it is not required for this guide. You can learn it as we go due
to it's simplicity. However, you may want to go through [Learn X in Y minutes
Where X=Go](https://learnxinyminutes.com/docs/go/) first to familiarize
yourself with the syntax.

By following along with this guide, you'll create a Tendermint Core project
called kvstore, a (very) simple distributed BFT key-value store.

## Built-in app vs external app

To get maximum performance it is better to run your application alongside
Tendermint Core. [Cosmos SDK](https://github.com/cosmos/cosmos-sdk) is written
this way. Please refer to [Writing a built-in Tendermint Core application in
Go](./go-built-in.md) guide for details.

Having a separate application might give you better security guarantees as two
processes would be communicating via established binary protocol. Tendermint
Core will not have access to application's state.

## 1.1 Installing Go

Please refer to [the official guide for installing
Go](https://go.dev/doc/install).

Verify that you have the latest version of Go installed:

```sh
$ go version
go version go1.18.x darwin/amd64
```

## 1.2 Creating a new Go project

We'll start by creating a new Go project. Initialize the folder with `go mod init`. Running this command should create the `go.mod` file.

```sh
$ mkdir kvstore
$ cd kvstore
$ go mod init github.com/<username>/kvstore
go: creating new go.mod: module github.com/<username>/kvstore
```

Inside the project directory, create a `main.go` file with the following content:

```go
package main

import (
 "fmt"
)

func main() {
 fmt.Println("Hello, Tendermint Core")
}
```

When run, this should print "Hello, Tendermint Core" to the standard output.

```sh
$ go run main.go
Hello, Tendermint Core
```

## 1.3 Writing a Tendermint Core application

Tendermint Core communicates with the application through the Application
BlockChain Interface (ABCI). All message types are defined in the [protobuf
file](https://github.com/tendermint/tendermint/blob/master/proto/tendermint/abci/types.proto).
This allows Tendermint Core to run applications written in any programming
language.

Create a file called `app.go` with the following content:

```go
package main

import (
 abcitypes "github.com/tendermint/tendermint/abci/types"
)

type KVStoreApplication struct {}

var _ abcitypes.Application = (*KVStoreApplication)(nil)

func NewKVStoreApplication() *KVStoreApplication {
 return &KVStoreApplication{}
}

func (KVStoreApplication) Info(req abcitypes.RequestInfo) abcitypes.ResponseInfo {
 return abcitypes.ResponseInfo{}
}

func (KVStoreApplication) DeliverTx(req abcitypes.RequestDeliverTx) abcitypes.ResponseDeliverTx {
 return abcitypes.ResponseDeliverTx{Code: 0}
}

func (KVStoreApplication) CheckTx(req abcitypes.RequestCheckTx) abcitypes.ResponseCheckTx {
 return abcitypes.ResponseCheckTx{Code: 0}
}

func (KVStoreApplication) Commit() abcitypes.ResponseCommit {
 return abcitypes.ResponseCommit{}
}

func (KVStoreApplication) Query(req abcitypes.RequestQuery) abcitypes.ResponseQuery {
 return abcitypes.ResponseQuery{Code: 0}
}

func (KVStoreApplication) InitChain(req abcitypes.RequestInitChain) abcitypes.ResponseInitChain {
 return abcitypes.ResponseInitChain{}
}

func (KVStoreApplication) BeginBlock(req abcitypes.RequestBeginBlock) abcitypes.ResponseBeginBlock {
 return abcitypes.ResponseBeginBlock{}
}

func (KVStoreApplication) EndBlock(req abcitypes.RequestEndBlock) abcitypes.ResponseEndBlock {
 return abcitypes.ResponseEndBlock{}
}

func (KVStoreApplication) ListSnapshots(abcitypes.RequestListSnapshots) abcitypes.ResponseListSnapshots {
 return abcitypes.ResponseListSnapshots{}
}

func (KVStoreApplication) OfferSnapshot(abcitypes.RequestOfferSnapshot) abcitypes.ResponseOfferSnapshot {
 return abcitypes.ResponseOfferSnapshot{}
}

func (KVStoreApplication) LoadSnapshotChunk(abcitypes.RequestLoadSnapshotChunk) abcitypes.ResponseLoadSnapshotChunk {
 return abcitypes.ResponseLoadSnapshotChunk{}
}

func (KVStoreApplication) ApplySnapshotChunk(abcitypes.RequestApplySnapshotChunk) abcitypes.ResponseApplySnapshotChunk {
 return abcitypes.ResponseApplySnapshotChunk{}
}
```

Now, we will go through each method and explain when it is executed while adding
required business logic.

### 1.3.1 Key-value store setup

For the underlying key-value store we'll use the latest version of [badger](https://github.com/dgraph-io/badger), which is an embeddable, persistent and fast key-value (KV) database.

```sh
$ go get github.com/dgraph-io/badger/v3
go: added github.com/dgraph-io/badger/v3 v3.2103.2
```

```go
import "github.com/dgraph-io/badger/v3"

type KVStoreApplication struct {
 db           *badger.DB
 currentBatch *badger.Txn
}

func NewKVStoreApplication(db *badger.DB) *KVStoreApplication {
 return &KVStoreApplication{
  db: db,
 }
}
```

### 1.3.2 CheckTx

When a new transaction is added to the Tendermint Core, it will ask the
application to check it (validate the format, signatures, etc.).

```go
import "bytes"

func (app *KVStoreApplication) isValid(tx []byte) (code uint32) {
 // check format
 parts := bytes.Split(tx, []byte("="))
 if len(parts) != 2 {
  return 1
 }

 key, value := parts[0], parts[1]

 // check if the same key=value already exists
 err := app.db.View(func(txn *badger.Txn) error {
  item, err := txn.Get(key)
  if err != nil && err != badger.ErrKeyNotFound {
   return err
  }
  if err == nil {
   return item.Value(func(val []byte) error {
    if bytes.Equal(val, value) {
     code = 2
    }
    return nil
   })
  }
  return nil
 })
 if err != nil {
  panic(err)
 }

 return code
}

func (app *KVStoreApplication) CheckTx(req abcitypes.RequestCheckTx) abcitypes.ResponseCheckTx {
 code := app.isValid(req.Tx)
 return abcitypes.ResponseCheckTx{Code: code, GasWanted: 1}
}
```

Don't worry if this does not compile yet.

If the transaction does not have a form of `{bytes}={bytes}`, we return `1`
code. When the same key=value already exist (same key and value), we return `2`
code. For others, we return a zero code indicating that they are valid.

Note that anything with non-zero code will be considered invalid (`-1`, `100`,
etc.) by Tendermint Core.

Valid transactions will eventually be committed given they are not too big and
have enough gas. To learn more about gas, check out ["the
specification"](https://github.com/tendermint/tendermint/blob/master/spec/abci/apps.md#gas).


### 1.3.3 BeginBlock -> DeliverTx -> EndBlock -> Commit

When Tendermint Core has decided on the block, it's transferred to the
application in 3 parts: `BeginBlock`, one `DeliverTx` per transaction and
`EndBlock` in the end. DeliverTx are being transferred asynchronously, but the
responses are expected to come in order.

```go
func (app *KVStoreApplication) BeginBlock(req abcitypes.RequestBeginBlock) abcitypes.ResponseBeginBlock {
 app.currentBatch = app.db.NewTransaction(true)
 return abcitypes.ResponseBeginBlock{}
}
```

Here we create a batch, which will store block's transactions.

```go
func (app *KVStoreApplication) DeliverTx(req abcitypes.RequestDeliverTx) abcitypes.ResponseDeliverTx {
 code := app.isValid(req.Tx)
 if code != 0 {
  return abcitypes.ResponseDeliverTx{Code: code}
 }

 parts := bytes.Split(req.Tx, []byte("="))
 key, value := parts[0], parts[1]

 err := app.currentBatch.Set(key, value)
 if err != nil {
  panic(err)
 }

 return abcitypes.ResponseDeliverTx{Code: 0}
}
```

If the transaction is badly formatted or the same key=value already exist, we
again return the non-zero code. Otherwise, we add it to the current batch.

In the current design, a block can include incorrect transactions (those who
passed CheckTx, but failed DeliverTx or transactions included by the proposer
directly). This is done for performance reasons.

Note we can't commit transactions inside the `DeliverTx` because in such case
`Query`, which may be called in parallel, will return inconsistent data (i.e.
it will report that some value already exist even when the actual block was not
yet committed).

`Commit` instructs the application to persist the new state.

```go
func (app *KVStoreApplication) Commit() abcitypes.ResponseCommit {
 app.currentBatch.Commit()
 return abcitypes.ResponseCommit{Data: []byte{}}
}
```

### 1.3.4 Query

Now, when the client wants to know whenever a particular key/value exist, it
will call Tendermint Core RPC `/abci_query` endpoint, which in turn will call
the application's `Query` method.

Applications are free to provide their own APIs. But by using Tendermint Core
as a proxy, clients (including [light client
package](https://godoc.org/github.com/tendermint/tendermint/light)) can leverage
the unified API across different applications. Plus they won't have to call the
otherwise separate Tendermint Core API for additional proofs.

Note we don't include a proof here.

```go
func (app *KVStoreApplication) Query(reqQuery abcitypes.RequestQuery) (resQuery abcitypes.ResponseQuery) {
 resQuery.Key = reqQuery.Data
 err := app.db.View(func(txn *badger.Txn) error {
  item, err := txn.Get(reqQuery.Data)
  if err != nil && err != badger.ErrKeyNotFound {
   return err
  }
  if err == badger.ErrKeyNotFound {
   resQuery.Log = "does not exist"
  } else {
   return item.Value(func(val []byte) error {
    resQuery.Log = "exists"
    resQuery.Value = val
    return nil
   })
  }
  return nil
 })
 if err != nil {
  panic(err)
 }
 return
}
```

The complete specification can be found
[here](https://github.com/tendermint/tendermint/tree/master/spec/abci/).

## 1.4 Starting an application and a Tendermint Core instances

Put the following code into the "main.go" file:

```go
package main

import (
 "flag"
 "fmt"
 "os"
 "os/signal"
 "syscall"

 "github.com/dgraph-io/badger/v3"

 abciserver "github.com/tendermint/tendermint/abci/server"
 "github.com/tendermint/tendermint/libs/log"
)

var socketAddr string

func init() {
 flag.StringVar(&socketAddr, "socket-addr", "unix://kvstore.sock", "Unix domain socket address")
}

func main() {
 db, err := badger.Open(badger.DefaultOptions("/tmp/badger"))
 if err != nil {
  fmt.Fprintf(os.Stderr, "failed to open badger db: %v", err)
  os.Exit(1)
 }
 defer db.Close()
 app := NewKVStoreApplication(db)

 flag.Parse()

 logger, err := log.NewDefaultLogger(log.LogFormatPlain, log.LogLevelInfo, false)
 if err != nil {
  fmt.Fprintf(os.Stderr, "failed to configure logger: %v", err)
  os.Exit(1)
 }

 server := abciserver.NewSocketServer(socketAddr, app)
 server.SetLogger(logger)
 if err := server.Start(); err != nil {
  fmt.Fprintf(os.Stderr, "error starting socket server: %v", err)
  os.Exit(1)
 }
 defer server.Stop()

 c := make(chan os.Signal, 1)
 signal.Notify(c, os.Interrupt, syscall.SIGTERM)
 <-c
}
```

This is a huge blob of code, so let's break it down into pieces.

First, we initialize the Badger database and create an app instance:

```go
db, err := badger.Open(badger.DefaultOptions("/tmp/badger"))
if err != nil {
 fmt.Fprintf(os.Stderr, "failed to open badger db: %v", err)
 os.Exit(1)
}
defer db.Close()
app := NewKVStoreApplication(db)
```

For **Windows** users, restarting this app will make badger throw an error as it requires value log to be truncated. For more information on this, visit [here](https://github.com/dgraph-io/badger/issues/744).
This can be avoided by setting the truncate option to true, like this:

```go
db, err := badger.Open(badger.DefaultOptions("/tmp/badger").WithTruncate(true))
```

Then we start the ABCI server and add some signal handling to gracefully stop
it upon receiving SIGTERM or Ctrl-C. Tendermint Core will act as a client,
which connects to our server and send us transactions and other messages.

```go
server := abciserver.NewSocketServer(socketAddr, app)
server.SetLogger(logger)
if err := server.Start(); err != nil {
 fmt.Fprintf(os.Stderr, "error starting socket server: %v", err)
 os.Exit(1)
}
defer server.Stop()

c := make(chan os.Signal, 1)
signal.Notify(c, os.Interrupt, syscall.SIGTERM)
<-c
```

## 1.5 Getting up and running

Make sure to enable [Go modules](https://github.com/golang/go/wiki/Modules). Run `go mod tidy` to download and add dependencies in `go.mod` file.

```sh
$ go mod tidy
...
```

Let's make sure we're using the latest version of Tendermint (currently `v0.35.8`).

```sh
$ go get github.com/tendermint/tendermint@latest
...
```

This will populate the `go.mod` with a release number followed by a hash for Tendermint.

```go
module github.com/<username>/kvstore

go 1.18

require (
 github.com/dgraph-io/badger/v3 v3.2103.2
 github.com/tendermint/tendermint v0.35.8
 ...
)
```

Now, we can build the binary:

```sh
$ go build
...
```

To create a default configuration, nodeKey and private validator files, let's
execute `tendermint init validator`. But before we do that, we will need to install
Tendermint Core. Please refer to [the official
guide](https://docs.tendermint.com/master/introduction/install.html). If you're
installing from source, don't forget to checkout the latest release (`git
checkout vX.Y.Z`). Don't forget to check that the application uses the same
major version.

```sh
$ rm -rf /tmp/kvstore /tmp/badger
$ TMHOME="/tmp/kvstore" tendermint init validator

2022-07-20T17:04:41+08:00 INFO Generated private validator keyFile=/tmp/kvstore/config/priv_validator_key.json module=main stateFile=/tmp/kvstore/data/priv_validator_state.json
2022-07-20T17:04:41+08:00 INFO Generated node key module=main path=/tmp/kvstore/config/node_key.json
2022-07-20T17:04:41+08:00 INFO Generated genesis file module=main path=/tmp/kvstore/config/genesis.json
2022-07-20T17:04:41+08:00 INFO Generated config mode=validator module=main
```

Feel free to explore the generated files, which can be found at
`/tmp/kvstore/config` directory. Documentation on the config can be found
[here](https://docs.tendermint.com/master/tendermint-core/configuration.html).

We are ready to start our application:

```sh
$ rm kvstore.sock
$ ./kvstore

badger 2022/07/20 17:07:17 INFO: All 1 tables opened in 9ms
badger 2022/07/20 17:07:17 INFO: Replaying file id: 0 at offset: 256
badger 2022/07/20 17:07:17 INFO: Replay took: 9.077µs
badger 2022/07/20 17:07:17 DEBUG: Value log discard stats empty
2022-07-20T17:07:17+08:00 INFO starting service impl=ABCIServer service=ABCIServer
2022-07-20T17:07:17+08:00 INFO Waiting for new connection...
```

Then, we need to start Tendermint Core and point it to our application. Staying
within the project directory, open another terminal and execute the command below:

```sh
$ TMHOME="/tmp/kvstore" tendermint node --proxy-app=unix://kvstore.sock

2022-07-20T17:10:22+08:00 INFO starting service impl=multiAppConn module=proxy service=multiAppConn
2022-07-20T17:10:22+08:00 INFO starting service connection=query impl=socketClient module=abci-client service=socketClient
2022-07-20T17:10:22+08:00 INFO starting service connection=snapshot impl=socketClient module=abci-client service=socketClient
2022-07-20T17:10:22+08:00 INFO starting service connection=mempool impl=socketClient module=abci-client service=socketClient
2022-07-20T17:10:22+08:00 INFO starting service connection=consensus impl=socketClient module=abci-client service=socketClient
2022-07-20T17:10:22+08:00 INFO starting service impl=EventBus module=events service=EventBus
2022-07-20T17:10:22+08:00 INFO starting service impl=PubSub module=pubsub service=PubSub
2022-07-20T17:10:22+08:00 INFO starting service impl=IndexerService module=txindex service=IndexerService
...
2022-07-20T17:10:22+08:00 INFO starting service impl=Node module=main service=Node
2022-07-20T17:10:22+08:00 INFO Starting RPC HTTP server on 127.0.0.1:26657 module=rpc-server
2022-07-20T17:10:22+08:00 INFO p2p service legacy_enabled=false module=main
2022-07-20T17:10:22+08:00 INFO starting service impl=router module=p2p service=router
2022-07-20T17:10:22+08:00 INFO starting router channels=402021222330386061626300 listen_addr=tcp://0.0.0.0:26656 module=p2p net_addr={"id":"715727499e94f8fcaef1763192ebcc8460f44666","ip":"0.0.0.0","port":26656} node_id=715727499e94f8fcaef1763192ebcc8460f44666
...
```

This should start the full node and connect to our ABCI application.

```sh
2022-07-20T17:09:55+08:00 INFO Waiting for new connection...
2022-07-20T17:10:22+08:00 INFO Accepted a new connection
2022-07-20T17:10:22+08:00 INFO Waiting for new connection...
2022-07-20T17:10:22+08:00 INFO Accepted a new connection
2022-07-20T17:10:22+08:00 INFO Waiting for new connection...
2022-07-20T17:10:22+08:00 INFO Accepted a new connection
```

Let's try sending a transaction. Open another terminal and execute the below command.

```sh
$ curl -s 'localhost:26657/broadcast_tx_commit?tx="tendermint=rocks"'
{
  ...
  "result": {
    "check_tx": {
      ...
      "gas_wanted": "1",
      ...
    },
    "deliver_tx": { ... },
    "hash": "1B3C5A1093DB952C331B1749A21DCCBB0F6C7F4E0055CD04D16346472FC60EC6",
    "height": "15"
  }
}
```

Response should contain the height where this transaction was committed.

Let's check if the given key now exists and its value:

```sh
$ curl -s 'localhost:26657/abci_query?data="tendermint"'
{
  ...
  "result": {
    "response": {
      "code": 0,
      "log": "exists",
      "info": "",
      "index": "0",
      "key": "dGVuZGVybWludA==",
      "value": "cm9ja3M=",
      "proofOps": null,
      "height": "0",
      "codespace": ""
    }
  }
}
```

`dGVuZGVybWludA==` and `cm9ja3M=` are the base64-encoding of the ASCII of
"tendermint" and "rocks" accordingly.

## Outro

I hope everything went smoothly and your first, but hopefully not the last,
Tendermint Core application is up and running. If not, please [open an issue on
Github](https://github.com/tendermint/tendermint/issues/new/choose). To dig
deeper, read [the docs](https://docs.tendermint.com/master/).
