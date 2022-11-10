<!---
order: 1
--->

# Creating an application in Go

## Guide Assumptions

This guide is designed for beginners who want to get started with a Tendermint
Core application from scratch. It does not assume that you have any prior
experience with Tendermint Core.

Tendermint Core is a service that provides a Byzantine Fault Tolerant consensus engine
for state-machine replication. The replicated state-machine, or "application", can be written
in any language that can send and receive protocol buffer messages in a client-server model.
Applications written in Go can also use Tendermint as a library and run the service in the same 
process as the application. 

By following along this tutorial you will create a Tendermint Core application called kvstore, 
a (very) simple distributed BFT key-value store.
The application will be written in Go and  
some understanding of the Go programming language is expected.
If you have never written Go, you may want to go through [Learn X in Y minutes
Where X=Go](https://learnxinyminutes.com/docs/go/) first, to familiarize
yourself with the syntax.

Note: Please use the latest released version of this guide and of Tendermint.
We strongly advise against using unreleased commits for your development.

### Built-in app vs external app
On the one hand, to get maximum performance you can run your application in 
the same process as the Tendermint Core, as long as your application is written in Go. 
[Cosmos SDK](https://github.com/cosmos/cosmos-sdk) is written
this way. 
If that is the way you wish to proceed, use the [Creating a built-in application in Go](./go-built-in.md) guide instead of this one.

On the other hand, having a separate application might give you better security 
guarantees as two processes would be communicating via established binary protocol. 
Tendermint Core will not have access to application's state. 
This is the approach followed in this tutorial.

## 1.1 Installing Go

Verify that you have the latest version of Go installed (refer to the [official guide for installing Go](https://golang.org/doc/install)):

```bash
$ go version
go version go1.19.2 darwin/amd64
```

## 1.2 Creating a new Go project

We'll start by creating a new Go project.

```bash
mkdir kvstore
```

Inside the example directory, create a `main.go` file with the following content:

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

```bash
cd kvstore
$ go run main.go
Hello, Tendermint Core
```

We are going to use [Go modules](https://github.com/golang/go/wiki/Modules) for
dependency management, so let's start by including a dependency on the latest version of
Tendermint.

```bash
go mod init kvstore
go get github.com/tendermint/tendermint@latest
```

After running the above commands you will see two generated files, `go.mod` and `go.sum`. 
The go.mod file should look similar to:

```go
module github.com/me/example

go 1.19

require (
	github.com/tendermint/tendermint v0.37.0
)
```

As you write the kvstore application, you can rebuild the binary by
pulling any new dependencies and recompiling it.

```sh
go get
go build
```


## 1.3 Writing a Tendermint Core application

Tendermint Core communicates with the application through the Application
BlockChain Interface (ABCI). The messages exchanged through the interface are 
defined in the ABCI [protobuf
file](https://github.com/tendermint/tendermint/blob/main/proto/tendermint/abci/types.proto).

We begin by creating the basic scaffolding for an ABCI application by 
creating a new type, `KVStoreApplication`, which implements the
methods defined by the `abcitypes.Application` interface.

Create a file called `app.go` with the following contents:

```go
package main

import (
	abcitypes "github.com/tendermint/tendermint/abci/types"
)

type KVStoreApplication struct{}

var _ abcitypes.Application = (*KVStoreApplication)(nil)

func NewKVStoreApplication() *KVStoreApplication {
	return &KVStoreApplication{}
}

func (app *KVStoreApplication) Info(info abcitypes.RequestInfo) abcitypes.ResponseInfo {
	return abcitypes.ResponseInfo{}
}

func (app *KVStoreApplication) Query(query abcitypes.RequestQuery) abcitypes.ResponseQuery {
	return abcitypes.ResponseQuery{}
}

func (app *KVStoreApplication) CheckTx(tx abcitypes.RequestCheckTx) abcitypes.ResponseCheckTx {
	return abcitypes.ResponseCheckTx{}
}

func (app *KVStoreApplication) InitChain(chain abcitypes.RequestInitChain) abcitypes.ResponseInitChain {
	return abcitypes.ResponseInitChain{}
}

func (app *KVStoreApplication) PrepareProposal(proposal abcitypes.RequestPrepareProposal) abcitypes.ResponsePrepareProposal {
	return abcitypes.ResponsePrepareProposal{}
}

func (app *KVStoreApplication) ProcessProposal(proposal abcitypes.RequestProcessProposal) abcitypes.ResponseProcessProposal {
	return abcitypes.ResponseProcessProposal{}
}

func (app *KVStoreApplication) BeginBlock(block abcitypes.RequestBeginBlock) abcitypes.ResponseBeginBlock {
	return abcitypes.ResponseBeginBlock{}
}

func (app *KVStoreApplication) DeliverTx(tx abcitypes.RequestDeliverTx) abcitypes.ResponseDeliverTx {
	return abcitypes.ResponseDeliverTx{}
}

func (app *KVStoreApplication) EndBlock(block abcitypes.RequestEndBlock) abcitypes.ResponseEndBlock {
	return abcitypes.ResponseEndBlock{}
}

func (app *KVStoreApplication) Commit() abcitypes.ResponseCommit {
	return abcitypes.ResponseCommit{}
}

func (app *KVStoreApplication) ListSnapshots(snapshots abcitypes.RequestListSnapshots) abcitypes.ResponseListSnapshots {
	return abcitypes.ResponseListSnapshots{}
}

func (app *KVStoreApplication) OfferSnapshot(snapshot abcitypes.RequestOfferSnapshot) abcitypes.ResponseOfferSnapshot {
	return abcitypes.ResponseOfferSnapshot{}
}

func (app *KVStoreApplication) LoadSnapshotChunk(chunk abcitypes.RequestLoadSnapshotChunk) abcitypes.ResponseLoadSnapshotChunk {
	return abcitypes.ResponseLoadSnapshotChunk{}
}

func (app *KVStoreApplication) ApplySnapshotChunk(chunk abcitypes.RequestApplySnapshotChunk) abcitypes.ResponseApplySnapshotChunk {
	return abcitypes.ResponseApplySnapshotChunk{}
}
```

The types used here are defined in the Tendermint library and were added as a dependency
to the project when you ran `go get`. If your IDE is not recognizing the types, go ahead and run the command again.

```bash
go get github.com/tendermint/tendermint@latest
```

Now go back to the `main.go` and modify the `main` function so it matches the following, 
where an instance of the `KVStoreApplication` type is created.

!!!!!!!!!!!

```go
func main() {
    fmt.Println("Hello, Tendermint Core")

    _ = NewKVStoreApplication()
}
```

You can recompile and run the application now by running `go get` and `go build`, but it does
not do anything.
So let's revisit the code adding the logic needed to implement our minimal key/value store
and to start it along with the Tendermint Service.


### 1.3.1 Add a persistent data store
Our application will need to write its state out to persistent storage so that it
can stop and start without losing all of its data.

For this tutorial, we will use [BadgerDB](https://github.com/dgraph-io/badger), a
a fast embedded key-value store. 

First, add Badger as a dependency of your go module using the `go get` command:

`go get github.com/dgraph-io/badger/v3`

Next, let's update the application and its constructor to receive a handle to the database, as follows: 

```go
type KVStoreApplication struct {
	db           *badger.DB
	onGoingBlock *badger.Txn
}

var _ abcitypes.Application = (*KVStoreApplication)(nil)

func NewKVStoreApplication(db *badger.DB) *KVStoreApplication {
	return &KVStoreApplication{db: db}
}
```

The `onGoingBlock` keeps track of the transaction that will update the application's state when a block 
is completed. Don't worry about it for now, we'll get to that later.

Next, update the `import` stanza at the top to include the Badger library:

```go
import(
	"github.com/dgraph-io/badger/v3"
	abcitypes "github.com/tendermint/tendermint/abci/types"
)
```

Finally, update the `main.go` file to invoke the updated constructor:

```go
	_ = NewKVStoreApplication(nil)
```


### 1.3.2 CheckTx
When Tendermint Core receives a new transaction from a client, Tendermint asks the application if 
the transaction is acceptable, using the `CheckTx` method.

In our application, a transaction is a string with the form `key=value`, indicating a key and value to write to the store.

The most basic validation check we can perform is to check if the transaction conforms to the `key=value` pattern.
For that, let's add the following helper method to app.go:

```go
func (app *KVStoreApplication) isValid(tx []byte) uint32 {
	// check format
	parts := bytes.Split(tx, []byte("="))
	if len(parts) != 2 {
		return 1
	}

	return 0
}
```

Now you can rewrite the `CheckTx` method to use the helper function:

```go
func (app *KVStoreApplication) CheckTx(req abcitypes.RequestCheckTx) abcitypes.ResponseCheckTx {
	code := app.isValid(req.Tx)
	return abcitypes.ResponseCheckTx{Code: code}
}
```

While this `CheckTx` is simple and only validates that the transaction is well-formed, 
it is very common for `CheckTx` to make more complex use of the state of an application.
For example, you may refuse to overwrite an existing value, or you can associate 
versions to the key/value pairs and allow the caller to specify a version to
perform a conditional update.

Depending on the checks and on the conditions violated, the function may return
different values, but any response with a non-zero code will be considered invalid 
by Tendermint. Our `CheckTx` logic returns 0 to Tendermint when a transaction passes 
its validation checks. The specific value of the code is meaningless to Tendermint. 
Non-zero codes are logged by Tendermint so applications can provide more specific 
information on why the transaction was rejected.

Note that `CheckTx` does not execute the transaction, it only verifies that that the transaction could be executed. We do not know yet if the rest of the network has agreed to accept this transaction into a block.


Finally, make sure to add the bytes package to the your `import` stanza at the top of `app.go`:

```go
import(
	"bytes"

	"github.com/dgraph-io/badger/v3"
	abcitypes "github.com/tendermint/tendermint/abci/types"
)
```


### 1.3.3 BeginBlock -> DeliverTx -> EndBlock -> Commit

When the Tendermint consensus engine has decided on the block, the block is transferred to the
application over three ABCI method calls: `BeginBlock`, `DeliverTx`, and `EndBlock`.

- `BeginBlock` is called once to indicate to the application that it is about to
receive a block.
- `DeliverTx` is called repeatedly, once for each application transaction that was included in the block.
- `EndBlock` is called once to indicate to the application that no more transactions
will be delivered to the application in within this block.

Note that, to implement these calls in our application we're going to make use of Badger's 
transaction mechanism. We will always refer to these as Badger transactions, not to
confuse them with the transactions included in the blocks delivered by Tendermint,
the _application transactions_.

First, let's create a new Badger transaction during `BeginBlock`. All application transactions in the
current block will be executed within this Badger transaction.
Then, return informing Tendermint that the application is ready to receive application transactions:

```go
func (app *KVStoreApplication) BeginBlock(req abcitypes.RequestBeginBlock) abcitypes.ResponseBeginBlock {
	app.onGoingBlock = app.db.NewTransaction(true)
	return abcitypes.ResponseBeginBlock{}
}
```

Next, let's modify `DeliverTx` to add the `key` and `value` to the database transaction every time our application
receives a new application transaction through `RequestDeliverTx`.

```go
func (app *KVStoreApplication) DeliverTx(req abcitypes.RequestDeliverTx) abcitypes.ResponseDeliverTx {
	if code := app.isValid(req.Tx); code != 0 {
		return abcitypes.ResponseDeliverTx{Code: code}
	}

	parts := bytes.SplitN(req.Tx, []byte("="), 2)
	key, value := parts[0], parts[1]

	if err := app.onGoingBlock.Set(key, value); err != nil {
		log.Panicf("Error reading database, unable to verify tx: %v", err)
	}

	return abcitypes.ResponseDeliverTx{Code: 0}
}
```

Note that we check the validity of the transaction _again_ during `DeliverTx`.
Transactions are not guaranteed to be valid when they are delivered to an
application, even if they were valid when they were proposed. 
This can happen if the application state is used to determine transaction
validity. Application state may have changed between the initial execution of `CheckTx`
and the transaction delivery in `DeliverTx` in a way that rendered the transaction
no longer valid.

`EndBlock` is called to inform the application that the full block has been delivered
and give the application a chance to perform any other computation needed, before the 
effects of the transactions become permanent.

Note that `EndBlock` **cannot** yet commit the Badger transaction we were building 
in during `DeliverTx`.
Since other methods, such as `Query`, rely on a consistent view of the application's
state, the application should only update its state by committing the Badger transactions 
when the full block has been delivered and the `Commit` method is invoked.

The `Commit` method tells the application to make permanent the effects of 
the application transactions.
Let's update the method to terminate the pending Badger transaction and
persist the resulting state: 

```go
func (app *KVStoreApplication) Commit() abcitypes.ResponseCommit {
	if err := app.onGoingBlock.Commit(); err != nil {
		log.Panicf("Error writing to database, unable to commit block: %v", err)
	}
	return abcitypes.ResponseCommit{Data: []byte{}}
}
```

Finally, make sure to add the log library to the `import` stanza as well:

```go
import (
	"bytes"
	"log"

	"github.com/dgraph-io/badger/v3"
	abcitypes "github.com/tendermint/tendermint/abci/types"
)
```

You may have noticed that the application we are writing will crash if it receives
an unexpected error from the Badger database during the `DeliverTx` or `Commit` methods.
This is not an accident. If the application received an error from the database, there 
is no deterministic way for it to make progress so the only safe option is to terminate.

### 1.3.4 Query
When a client tries to read some information from the `kvstore`, the request will be
handled in the `Query` method. To do this, let's rewrite the `Query` method in `app.go`:

```go
func (app *KVStoreApplication) Query(req abcitypes.RequestQuery) abcitypes.ResponseQuery {
	resp := abcitypes.ResponseQuery{Key: req.Data}

	dbErr := app.db.View(func(txn *badger.Txn) error {
		item, err := txn.Get(req.Data)
		if err != nil {
			if err != badger.ErrKeyNotFound {
				return err
			}
			resp.Log = "key does not exist"
			return nil
		}

		return item.Value(func(val []byte) error {
			resp.Log = "exists"
			resp.Value = val
			return nil
		})
	})
	if dbErr != nil {
		log.Panicf("Error reading database, unable to verify tx: %v", dbErr)
	}
	return resp
}
```

Since it reads only committed data from the store, transactions that are part of a block
that is still processed are not reflected in the query result.

### 1.3.5 PrepareProposal and ProcessProposal
`PrepareProposal` and `ProcessProposal` are methods introduced in Tendermint 3.7.0 
to give the application more control over the construction and processing of transaction blocks.

When Tendermint Core sees that valid transactions (validated through `CheckTx`) are available to be
included in blocks, it groups some of these transactions and then gives the application a chance 
to modify the group by invoking `PrepareProposal`.

The application is free to modify the group before returning from the call.
For example, the application may reorder or even remove transactions from the group to improve the
execution of the block once accepted.
In the following code, the application simply returns the unmodified group of transactions:

```go
func (app *KVStoreApplication) PrepareProposal(proposal abcitypes.RequestPrepareProposal) abcitypes.ResponsePrepareProposal {
	return abcitypes.ResponsePrepareProposal{Txs: proposal.Txs}
}
```

Once a proposed block is received by a node, the proposal is passed to the application to give
its blessing before voting to accept the proposal.

This mechanism may be used for different reasons, for example to deal with blocks manipulated 
by malicious nodes, in which case the block should not be considered valid.
The following code simply accepts all proposals:

```go
func (app *KVStoreApplication) ProcessProposal(proposal abcitypes.RequestProcessProposal) abcitypes.ResponseProcessProposal {
	return abcitypes.ResponseProcessProposal{Status: abcitypes.ResponseProcessProposal_ACCEPT}
}
```

## 1.4 Starting an application and a Tendermint Core instances

Now that we have the basic functionality of our application in place, let's put it all together inside of our main.go file.

Change the contents of your`main.go` file to the following.

```go
package main

import (
	"flag"
	"fmt"
	abciserver "github.com/tendermint/tendermint/abci/server"
	"log"
	"os"
	"os/signal"
	"path/filepath"
	"syscall"

	"github.com/dgraph-io/badger/v3"
	tmlog "github.com/tendermint/tendermint/libs/log"
)

var homeDir string
var socketAddr string

func init() {
	flag.StringVar(&homeDir, "kv-home", "", "Path to the kvstore directory (if empty, uses $HOME/.kvstore)")
	flag.StringVar(&socketAddr, "socket-addr", "unix://example.sock", "Unix domain socket address (if empty, uses \"unix://example.sock\"")
}

func main() {
	flag.Parse()
	if homeDir == "" {
		homeDir = os.ExpandEnv("$HOME/.kvstore")
	}

	dbPath := filepath.Join(homeDir, "badger")
	db, err := badger.Open(badger.DefaultOptions(dbPath))
	if err != nil {
		log.Fatalf("Opening database: %v", err)
	}
	defer func() {
		if err := db.Close(); err != nil {
			log.Fatalf("Closing database: %v", err)
		}
	}()

	app := NewKVStoreApplication(db)

	logger := tmlog.NewTMLogger(tmlog.NewSyncWriter(os.Stdout))

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
	dbPath := filepath.Join(homeDir, "badger")
	db, err := badger.Open(badger.DefaultOptions(dbPath))
	if err != nil {
		log.Fatalf("Opening database: %v", err)
	}
	defer func() {
		if err := db.Close(); err != nil {
			log.Fatalf("Closing database: %v", err)
		}
	}()

	app := NewKVStoreApplication(db)
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
## 1.5 Initializing and Running

Our application is almost ready to run, but first we'll need to populate the Tendermint Core configuration files.
The following command will create a `tendermint-home` directory in your project and add a basic set of configuration files in `tendermint-home/config/`.
For more information on what these files contain see [the configuration documentation](https://github.com/tendermint/tendermint/blob/v0.37.0/docs/nodes/configuration.md).

From the root of your project, run:

```bash
go run github.com/tendermint/tendermint/cmd/tendermint@v0.37.0 init --home /tmp/tendermint-home
```

You should see an output similar to the following:

```bash
I[2022-11-09|09:06:34.444] Generated private validator                  module=main keyFile=/tmp/tendermint-home/config/priv_validator_key.json stateFile=/tmp/tendermint-home/data/priv_validator_state.json
I[2022-11-09|09:06:34.444] Generated node key                           module=main path=/tmp/tendermint-home/config/node_key.json
I[2022-11-09|09:06:34.444] Generated genesis file                       module=main path=/tmp/tendermint-home/config/genesis.json
```

Now rebuild the app:

```bash
go build -mod=mod # use -mod=mod to automatically refresh the dependencies
```

Everything is now in place to run your application. Run:

```bash
./kvstore -kv-home /tmp/badger-home
```

The application will start and you should see an output similar to the following:

```bash
badger 2022/11/09 17:01:28 INFO: All 0 tables opened in 0s
badger 2022/11/09 17:01:28 INFO: Discard stats nextEmptySlot: 0
badger 2022/11/09 17:01:28 INFO: Set nextTxnTs to 0
I[2022-11-09|17:01:28.726] service start                                msg="Starting ABCIServer service" impl=ABCIServer
I[2022-11-09|17:01:28.726] Waiting for new connection...
```

Then we need to start Tendermint Core service and point it to our application. 
Open a new terminal window and cd to the same folder where the app is running. 
Then execute the following command:

```bash
go run github.com/tendermint/tendermint/cmd/tendermint@v0.37.0 node --home /tmp/tendermint-home --proxy_app=unix://example.sock
```

This should start the full node and connect to our ABCI application, which will be
reflected in the application output.

```sh
I[2022-11-09|17:07:08.124] service start                                msg="Starting ABCIServer service" impl=ABCIServer
I[2022-11-09|17:07:08.124] Waiting for new connection...
I[2022-11-09|17:08:12.702] Accepted a new connection
I[2022-11-09|17:08:12.703] Waiting for new connection...
I[2022-11-09|17:08:12.703] Accepted a new connection
I[2022-11-09|17:08:12.703] Waiting for new connection...
```

Also, the application using Tendermint Core is producing blocks  🎉🎉 and you can see this reflected in the log output of the service in lines like this:

```bash
I[2022-11-09|09:08:52.147] received proposal                            module=consensus proposal="Proposal{2/0 (F518444C0E348270436A73FD0F0B9DFEA758286BEB29482F1E3BEA75330E825C:1:C73D3D1273F2, -1) AD19AE292A45 @ 2022-11-09T12:08:52.143393Z}"
I[2022-11-09|09:08:52.152] received complete proposal block             module=consensus height=2 hash=F518444C0E348270436A73FD0F0B9DFEA758286BEB29482F1E3BEA75330E825C
I[2022-11-09|09:08:52.160] finalizing commit of block                   module=consensus height=2 hash=F518444C0E348270436A73FD0F0B9DFEA758286BEB29482F1E3BEA75330E825C root= num_txs=0
I[2022-11-09|09:08:52.167] executed block                               module=state height=2 num_valid_txs=0 num_invalid_txs=0
I[2022-11-09|09:08:52.171] committed state                              module=state height=2 num_txs=0 app_hash=
```

The blocks, as you can see from the `num_valid_txs=0` part, are empty, but let's remedy that next.

## 1.6 Using the application

Let's try submitting a transaction to our new application.
Open another terminal window and run the following curl command:


```bash
curl -s 'localhost:26657/broadcast_tx_commit?tx="tendermint=rocks"'
```
If everything went well, you should see a response indicating which height the 
+transaction was included in the blockchain.

Finally, let's make sure that transaction really was persisted by the application.
Run the following command:

```bash
curl -s 'localhost:26657/abci_query?data="tendermint"'
```

Let's examine the response object that this request returns.
The request returns a `json` object with a `key` and `value` field set.

```json
...
	"key": "dGVuZGVybWludA==",
	"value": "cm9ja3M=",
...
```

Those values don't look like the `key` and `value` we sent to Tendermint.
What's going on here? 

The response contain a `base64` encoded representation of the data we submitted.
To get the original value out of this data, we can use the `base64` command line utility:

```bash
echo cm9ja3M=" | base64 -d
```

## Outro

I hope everything went smoothly and your first, but hopefully not the last,
Tendermint Core application is up and running. If not, please [open an issue on
Github](https://github.com/tendermint/tendermint/issues/new/choose). To dig
deeper, read [the docs](https://docs.tendermint.com/main/).
