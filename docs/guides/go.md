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
Go](https://golang.org/doc/install).

Verify that you have the latest version of Go installed:

```sh
$ go version
go version go1.12.7 darwin/amd64
```

Make sure you have `$GOPATH` environment variable set:

```sh
$ echo $GOPATH
/Users/melekes/go
```

## 1.2 Creating a new Go project

We'll start by creating a new Go project.

```sh
$ mkdir -p $GOPATH/src/github.com/me/kvstore
$ cd $GOPATH/src/github.com/me/kvstore
```

Inside the example directory create a `main.go` file with the following content:

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
file](https://github.com/tendermint/tendermint/blob/develop/abci/types/types.proto).
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

func (KVStoreApplication) SetOption(req abcitypes.RequestSetOption) abcitypes.ResponseSetOption {
	return abcitypes.ResponseSetOption{}
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
```

Now I will go through each method explaining when it's called and adding
required business logic.

### 1.3.1 CheckTx

When a new transaction is added to the Tendermint Core, it will ask the
application to check it (validate the format, signatures, etc.).

```go
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
specification"](https://tendermint.com/docs/spec/abci/apps.html#gas).

For the underlying key-value store we'll use
[badger](https://github.com/dgraph-io/badger), which is an embeddable,
persistent and fast key-value (KV) database.

```go
import "github.com/dgraph-io/badger"

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

### 1.3.2 BeginBlock -> DeliverTx -> EndBlock -> Commit

When Tendermint Core has decided on the block, it's transfered to the
application in 3 parts: `BeginBlock`, one `DeliverTx` per transaction and
`EndBlock` in the end. DeliverTx are being transfered  asynchronously, but the
responses are expected to come in order.

```
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

### 1.3.3 Query

Now, when the client wants to know whenever a particular key/value exist, it
will call Tendermint Core RPC `/abci_query` endpoint, which in turn will call
the application's `Query` method.

Applications are free to provide their own APIs. But by using Tendermint Core
as a proxy, clients (including [light client
package](https://godoc.org/github.com/tendermint/tendermint/lite)) can leverage
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
[here](https://tendermint.com/docs/spec/abci/).

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

	"github.com/dgraph-io/badger"

	abciserver "github.com/tendermint/tendermint/abci/server"
	"github.com/tendermint/tendermint/libs/log"
)

var socketAddr string

func init() {
	flag.StringVar(&socketAddr, "socket-addr", "unix://example.sock", "Unix domain socket address")
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

	logger := log.NewTMLogger(log.NewSyncWriter(os.Stdout))

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
	os.Exit(0)
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
os.Exit(0)
```

## 1.5 Getting Up and Running

We are going to use [Go modules](https://github.com/golang/go/wiki/Modules) for
dependency management.

```sh
$ export GO111MODULE=on
$ go mod init github.com/me/example
$ go build
```

This should build the binary.

To create a default configuration, nodeKey and private validator files, let's
execute `tendermint init`. But before we do that, we will need to install
Tendermint Core.

```sh
$ rm -rf /tmp/example
$ cd $GOPATH/src/github.com/tendermint/tendermint
$ make install
$ TMHOME="/tmp/example" tendermint init

I[2019-07-16|18:20:36.480] Generated private validator                  module=main keyFile=/tmp/example/config/priv_validator_key.json stateFile=/tmp/example2/data/priv_validator_state.json
I[2019-07-16|18:20:36.481] Generated node key                           module=main path=/tmp/example/config/node_key.json
I[2019-07-16|18:20:36.482] Generated genesis file                       module=main path=/tmp/example/config/genesis.json
```

Feel free to explore the generated files, which can be found at
`/tmp/example/config` directory. Documentation on the config can be found
[here](https://tendermint.com/docs/tendermint-core/configuration.html).

We are ready to start our application:

```sh
$ rm example.sock
$ ./example

badger 2019/07/16 18:25:11 INFO: All 0 tables opened in 0s
badger 2019/07/16 18:25:11 INFO: Replaying file id: 0 at offset: 0
badger 2019/07/16 18:25:11 INFO: Replay took: 300.4s
I[2019-07-16|18:25:11.523] Starting ABCIServer                          impl=ABCIServ
```

Then we need to start Tendermint Core and point it to our application. Staying
within the application directory execute:

```sh
$ TMHOME="/tmp/example" tendermint node --proxy_app=unix://example.sock

I[2019-07-16|18:26:20.362] Version info                                 module=main software=0.32.1 block=10 p2p=7
I[2019-07-16|18:26:20.383] Starting Node                                module=main impl=Node
E[2019-07-16|18:26:20.392] Couldn't connect to any seeds                module=p2p
I[2019-07-16|18:26:20.394] Started node                                 module=main nodeInfo="{ProtocolVersion:{P2P:7 Block:10 App:0} ID_:8dab80770ae8e295d4ce905d86af78c4ff634b79 ListenAddr:tcp://0.0.0.0:26656 Network:test-chain-nIO96P Version:0.32.1 Channels:4020212223303800 Moniker:app48.fun-box.ru Other:{TxIndex:on RPCAddress:tcp://127.0.0.1:26657}}"
I[2019-07-16|18:26:21.440] Executed block                               module=state height=1 validTxs=0 invalidTxs=0
I[2019-07-16|18:26:21.446] Committed state                              module=state height=1 txs=0 appHash=
```

This should start the full node and connect to our ABCI application.

```
I[2019-07-16|18:25:11.525] Waiting for new connection...
I[2019-07-16|18:26:20.329] Accepted a new connection
I[2019-07-16|18:26:20.329] Waiting for new connection...
I[2019-07-16|18:26:20.330] Accepted a new connection
I[2019-07-16|18:26:20.330] Waiting for new connection...
I[2019-07-16|18:26:20.330] Accepted a new connection
```

Now open another tab in your terminal and try sending a transaction:

```sh
$ curl -s 'localhost:26657/broadcast_tx_commit?tx="tendermint=rocks"'
{
  "jsonrpc": "2.0",
  "id": "",
  "result": {
    "check_tx": {
      "gasWanted": "1"
    },
    "deliver_tx": {},
    "hash": "CDD3C6DFA0A08CAEDF546F9938A2EEC232209C24AA0E4201194E0AFB78A2C2BB",
    "height": "33"
}
```

Response should contain the height where this transaction was committed.

Now let's check if the given key now exists and its value:

```
$ curl -s 'localhost:26657/abci_query?data="tendermint"'
{
  "jsonrpc": "2.0",
  "id": "",
  "result": {
    "response": {
      "log": "exists",
      "key": "dGVuZGVybWludA==",
      "value": "cm9ja3My"
    }
  }
}
```

"dGVuZGVybWludA==" and "cm9ja3M=" are the base64-encoding of the ASCII of
"tendermint" and "rocks" accordingly.

## Outro

I hope everything went smoothly and your first, but hopefully not the last,
Tendermint Core application is up and running. If not, please [open an issue on
Github](https://github.com/tendermint/tendermint/issues/new/choose). To dig
deeper, read [the docs](https://tendermint.com/docs/).
