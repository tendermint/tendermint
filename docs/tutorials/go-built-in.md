<!---
order: 2
--->

# Creating a built-in application in Go

## Guide assumptions

This guide is designed for beginners who want to get started with a Tendermint
Core application from scratch. It does not assume that you have any prior
experience with Tendermint Core.

Tendermint Core is a service that provides a Byzantine fault tolerant consensus engine
for state-machine replication. The replicated state-machine, or "application", can be written
in any language that can send and receive protocol buffer messages.

This tutorial is written for Go and uses Tendermint as a library, but applications not
written in Go can use Tendermint to drive state-machine replication in a client-server
model.

This tutorial expects some understanding of the Go programming language.
If you have never written Go, you may want to go through [Learn X in Y minutes
Where X=Go](https://learnxinyminutes.com/docs/go/) first to familiarize
yourself with the syntax.

By following along with this guide, you'll create a Tendermint Core application
called kvstore, a (very) simple distributed BFT key-value store.

> Note: please use a released version of Tendermint with this guide. The guides will work with the latest version. Please, do not use master.

## Built-in app vs external app

Running your application inside the same process as Tendermint Core will give
you the best possible performance.

For other languages, your application have to communicate with Tendermint Core
through a TCP, Unix domain socket or gRPC.

## 1.1 Installing Go

Please refer to [the official guide for installing
Go](https://golang.org/doc/install).

Verify that you have the latest version of Go installed:

```bash
$ go version
go version go1.17.5 darwin/amd64
```

Note that the exact patch number may differ as Go releases come out.
## 1.2 Creating a new Go project

We'll start by creating a new Go project.

```bash
mkdir kvstore
cd kvstore
go mod init github.com/<github_username>/<repo_name>
```

Inside the example directory create a `main.go` file with the following content:

> Note: there is no need to clone or fork Tendermint in this tutorial.

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
$ go run main.go
Hello, Tendermint Core
```

## 1.3 Writing a Tendermint Core application

Tendermint Core communicates with an application through the Application
BlockChain Interface (ABCI) protocol. All of the message types Tendermint uses for
communicating with the application can be found in the ABCI [protobuf
file](https://github.com/tendermint/spec/blob/b695d30aae69933bc0e630da14949207d18ae02c/proto/tendermint/abci/types.proto).

We will begin by creating the basic scaffolding for an ABCI application in
a new `app.go` file. The first step is to create a new type, `KVStoreApplication`
with methods that implement the abci `Application` interface.

Create a file called `app.go` and add the following contents:

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

func (app *KVStoreApplication) Info(req abcitypes.RequestInfo) abcitypes.ResponseInfo {
 return abcitypes.ResponseInfo{}
}

func (app *KVStoreApplication) DeliverTx(req abcitypes.RequestDeliverTx) abcitypes.ResponseDeliverTx {
 return abcitypes.ResponseDeliverTx{Code: 0}
}

func (app *KVStoreApplication) CheckTx(req abcitypes.RequestCheckTx) abcitypes.ResponseCheckTx {
 return abcitypes.ResponseCheckTx{Code: 0}
}

func (app *KVStoreApplication) Commit() abcitypes.ResponseCommit {
 return abcitypes.ResponseCommit{}
}

func (app *KVStoreApplication) Query(req abcitypes.RequestQuery) abcitypes.ResponseQuery {
 return abcitypes.ResponseQuery{Code: 0}
}

func (app *KVStoreApplication) InitChain(req abcitypes.RequestInitChain) abcitypes.ResponseInitChain {
 return abcitypes.ResponseInitChain{}
}

func (app *KVStoreApplication) BeginBlock(req abcitypes.RequestBeginBlock) abcitypes.ResponseBeginBlock {
 return abcitypes.ResponseBeginBlock{}
}

func (app *KVStoreApplication) EndBlock(req abcitypes.RequestEndBlock) abcitypes.ResponseEndBlock {
 return abcitypes.ResponseEndBlock{}
}

func (app *KVStoreApplication) ListSnapshots(abcitypes.RequestListSnapshots) abcitypes.ResponseListSnapshots {
 return abcitypes.ResponseListSnapshots{}
}

func (app *KVStoreApplication) OfferSnapshot(abcitypes.RequestOfferSnapshot) abcitypes.ResponseOfferSnapshot {
 return abcitypes.ResponseOfferSnapshot{}
}

func (app *KVStoreApplication) LoadSnapshotChunk(abcitypes.RequestLoadSnapshotChunk) abcitypes.ResponseLoadSnapshotChunk {
 return abcitypes.ResponseLoadSnapshotChunk{}
}

func (app *KVStoreApplication) ApplySnapshotChunk(abcitypes.RequestApplySnapshotChunk) abcitypes.ResponseApplySnapshotChunk {
 return abcitypes.ResponseApplySnapshotChunk{}
}
```

### 1.3.1 Add a persistent data store

Our application will need to write its state out to persistent storage so that it
can stop and start without losing all of its data.

For this tutorial, we will use [BadgerDB](https://github.com/dgraph-io/badger).
Badger is a fast embedded key-value store.

First, add Badger as a dependency of your go module using the `go get` command:

`go get github.com/dgraph-io/badger/v3`

Next, let's update the application and its constructor to receive a handle to the
database.

Update the application struct as follows:

```go
type KVStoreApplication struct {
	db           *badger.DB
	pendingBlock *badger.Txn
}
```

And change the constructor to set the appropriate field when creating the application:

```go
func NewKVStoreApplication(db *badger.DB) *KVStoreApplication {
	return &KVStoreApplication{db: db}
}
```

The `pendingBlock` keeps track of the transactions that will update the application's
state when a block is completed. Don't worry about it for now, we'll get to that later.

Finally, update the `import` stanza at the top to include the `Badger` library:

```go
import(
	"github.com/dgraph-io/badger/v3"
	abcitypes "github.com/tendermint/tendermint/abci/types"
)
```

### 1.3.1 CheckTx

When Tendermint Core receives a new transaction, Tendermint asks the application
if the transaction is acceptable. In our new application, let's implement some
basic validation for the transactions it will receive.

For our KV store application, a transaction is a string with the form `key=value`,
indicating a key and value to write to the store.

Add the following helper method to `app.go`:

```go
func (app *KVStoreApplication) validateTx(tx []byte) uint32 {
	parts := bytes.SplitN(tx, []byte("="), 2)

	// check that the transaction is not malformed
	if len(parts) != 2 || len(parts[0]) == 0 {
		return 1
	}
	return 0
}
```

And call it from within your `CheckTx` method:

```go
func (app *KVStoreApplication) CheckTx(req abcitypes.RequestCheckTx) abcitypes.ResponseCheckTx {
	code := app.validateTx(req.Tx)
	return abcitypes.ResponseCheckTx{Code: code}
}
```

Any response with a non-zero code will be considered invalid by Tendermint.
Our `CheckTx` logic returns `0` to Tendermint when a transaction passes
its validation checks. The specific value of the code is meaningless to Tendermint.
Non-zero codes are logged by Tendermint so applications can provide more specific
information on why the transaction was rejected.

Note that `CheckTx` _does not execute_ the transaction, it only verifies that that the
transaction _could_ be executed. We do not know yet if the rest of the network has
agreed to accept this transaction into a block.

Valid transactions will eventually be committed given they are not too big and
have enough gas. To learn more about gas, check out ["the
specification"](https://github.com/tendermint/tendermint/blob/master/spec/abci/apps.md#gas).

For the underlying key-value store we'll use
[badger](https://github.com/dgraph-io/badger), which is an embeddable,
persistent and fast key-value (KV) database.

```go
import(
	"bytes"

	"github.com/dgraph-io/badger/v3"
	abcitypes "github.com/tendermint/tendermint/abci/types"
)
```


While this `CheckTx` is simple and only validates that the transaction is well-formed,
it is very common for `CheckTx` to make more complex use of the state of an application.

### 1.3.2 BeginBlock -> DeliverTx -> EndBlock -> Commit

When the Tendermint consensus engine has decided on the block, the block is transferred to the
application over three ABCI method calls: `BeginBlock`, `DeliverTx`, and `EndBlock`.

`BeginBlock` is called once to indicate to the application that it is about to
receive a block.

`DeliverTx` is called repeatedly, once for each `Tx` that was included in the block.

`EndBlock` is called once to indicate to the application that no more transactions
will be delivered to the application.

To implement these calls in our application we're going to make use of Badger's
transaction mechanism. Bagder uses the term _transaction_ in the context of databases,
be careful not to confuse it with _blockchain transactions_.

First, let's create a new Badger `Txn` during `BeginBlock`:

```go
func (app *KVStoreApplication) BeginBlock(req abcitypes.RequestBeginBlock) abcitypes.ResponseBeginBlock {
	app.pendingBlock = app.db.NewTransaction(true)
	return abcitypes.ResponseBeginBlock{}
}
```

Next, let's modify `DeliverTx` to add the `key` and `value` to the database `Txn` every time our application
receives a new `RequestDeliverTx`.

```go
func (app *KVStoreApplication) DeliverTx(req abcitypes.RequestDeliverTx) abcitypes.ResponseDeliverTx {
	if code := app.validateTx(req.Tx); code != 0 {
		return abcitypes.ResponseDeliverTx{Code: code}
	}

	parts := bytes.SplitN(req.Tx, []byte("="), 2)
	key, value := parts[0], parts[1]

	if err := app.pendingBlock.Set(key, value); err != nil {
		log.Panicf("Error reading database, unable to verify tx: %v", err)
	}

	return abcitypes.ResponseDeliverTx{Code: 0}
}
```
Note that we check the validity of the transaction _again_ during `DeliverTx`.
Transactions are not guaranteed to be valid when they are delivered to an
application. This can happen if the application state is used to determine transaction
validity. Application state may have changed between when the `CheckTx` was initially
called and when the transaction was delivered in `DeliverTx` in a way that rendered
the transaction no longer valid.

Also note that we don't commit the Badger `Txn` we are building during `DeliverTx`.
Other methods, such as `Query`, rely on a consistent view of the application's state.
The application should only update its state when the full block has been delivered.

The `Commit` method indicates that the full block has been delivered. During `Commit`,
the application should persist the pending `Txn`.

Let's modify our `Commit` method to persist the new state to the database:

```go
func (app *KVStoreApplication) Commit() abcitypes.ResponseCommit {
	if err := app.pendingBlock.Commit(); err != nil {
		log.Panicf("Error writing to database, unable to commit block: %v", err)
	}
	return abcitypes.ResponseCommit{Data: []byte{}}
}
```

Finally, make sure to add the `log` library to the `import` stanza as well:

```go
import (
	"bytes"
	"log"

	"github.com/dgraph-io/badger/v3"
	abcitypes "github.com/tendermint/tendermint/abci/types"
)
```

You may have noticed that the application we are writing will _crash_ if it receives an
unexpected error from the database during the `DeliverTx` or `Commit` methods.
This is not an accident. If the application received an error from the database,
there is no deterministic way for it to make progress so the only safe option is to terminate.

### 1.3.3 Query Method

We'll want to be able to determine if a transaction was committed to the state-machine.
To do this, let's implement the `Query` method in `app.go`:

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

The complete specification can be found
[here](https://github.com/tendermint/tendermint/tree/master/spec/abci/).

## 1.4 Starting an application and a Tendermint Core instance in the same process

Now that we have the basic functionality of our application in place, let's put it
all together inside of our `main.go` file.

Add the following code to your `main.go` file:

```go
package main

import (
	"flag"
	"fmt"
	"log"
	"os"
	"os/signal"
	"path/filepath"
	"syscall"

	"github.com/dgraph-io/badger/v3"
	"github.com/spf13/viper"
	abciclient "github.com/tendermint/tendermint/abci/client"
	cfg "github.com/tendermint/tendermint/config"
	tmlog "github.com/tendermint/tendermint/libs/log"
	nm "github.com/tendermint/tendermint/node"
	"github.com/tendermint/tendermint/types"
)

var homeDir string

func init() {
	flag.StringVar(&homeDir, "tm-home", "", "Path to the tendermint config directory (if empty, uses $HOME/.tendermint)")
}

func main() {
	flag.Parse()
	if homeDir == "" {
		homeDir = os.ExpandEnv("$HOME/.tendermint")
	}
	config := cfg.DefaultValidatorConfig()

	config.SetRoot(homeDir)

	viper.SetConfigFile(fmt.Sprintf("%s/%s", homeDir, "config/config.toml"))
	if err := viper.ReadInConfig(); err != nil {
		log.Fatalf("Reading config: %v", err)
	}
	if err := viper.Unmarshal(config); err != nil {
		log.Fatalf("Decoding config: %v", err)
	}
	if err := config.ValidateBasic(); err != nil {
		log.Fatalf("Invalid configuration data: %v", err)
	}
	gf, err := types.GenesisDocFromFile(config.GenesisFile())
	if err != nil {
		log.Fatalf("Loading genesis document: %v", err)
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
	acc := abciclient.NewLocalCreator(app)

	logger := tmlog.MustNewDefaultLogger(tmlog.LogFormatPlain, tmlog.LogLevelInfo, false)
	node, err := nm.New(config, logger, acc, gf)
	if err != nil {
		log.Fatalf("Creating node: %v", err)
	}

	node.Start()
	defer func() {
		node.Stop()
		node.Wait()
	}()

	c := make(chan os.Signal, 1)
	signal.Notify(c, os.Interrupt, syscall.SIGTERM)
	<-c
}
```

This is a huge blob of code, so let's break down what it's doing.

First, we load in the Tendermint Core configuration files:

```go
...
	config := cfg.DefaultValidatorConfig()

	config.SetRoot(homeDir)

	viper.SetConfigFile(fmt.Sprintf("%s/%s", homeDir, "config/config.toml"))
	if err := viper.ReadInConfig(); err != nil {
		log.Fatalf("Reading config: %v", err)
	}
	if err := viper.Unmarshal(config); err != nil {
		log.Fatalf("Decoding config: %v", err)
	}
	if err := config.ValidateBasic(); err != nil {
		log.Fatalf("Invalid configuration data: %v", err)
	}
	gf, err := types.GenesisDocFromFile(config.GenesisFile())
	if err != nil {
		log.Fatalf("Loading genesis document: %v", err)
	}
...
```

Next, we create a database handle and use it to construct our ABCI application:

```go
...
	dbPath := filepath.Join(homeDir, "badger")
	db, err := badger.Open(badger.DefaultOptions(dbPath).WithTruncate(true))
	if err != nil {
		log.Fatalf("Opening database: %v", err)
	}
	defer func() {
		if err := db.Close(); err != nil {
			log.Fatalf("Error closing database: %v", err)
		}
	}()
	app := NewKVStoreApplication(db)
	acc := abciclient.NewLocalCreator(app)
...
```

Then we construct a logger:
```go
...
	logger := tmlog.MustNewDefaultLogger(tmlog.LogFormatPlain, tmlog.LogLevelInfo, false)
...
```

Now we have everything setup to run the Tendermint node. We construct
a node by passing it the configuration, the logger, a handle to our application and
the genesis file:

```go
...
	node, err := nm.New(config, logger, acc, gf)
	if err != nil {
		log.Fatalf("Creating node: %v", err)
	}
...
```

Finally, we start the node:
```go
...
	node.Start()
	defer func() {
		node.Stop()
		node.Wait()
	}()
...
```

The additional logic at the end of the file allows the program to catch `SIGTERM`.
This means that the node can shutdown gracefully when an operator tries to kill the program:

```go
...
c := make(chan os.Signal, 1)
signal.Notify(c, os.Interrupt, syscall.SIGTERM)
<-c
...
```

## 1.5 Getting Up and Running

Our application is almost ready to run.
Let's install the latest release version of the Tendermint library.

From inside of the project directory, run:

```sh
go get github.com/dashevo/tenderdash@master
```

Next, we'll need to populate the Tendermint Core configuration files.
This command will create a `tendermint-home` directory in your project and add a basic set of configuration
files in `tendermint-home/config/`. For more information on what these files contain
see [the configuration documentation](https://github.com/tendermint/tendermint/blob/v0.35.0/docs/nodes/configuration.md).

From the root of your project, run:
```bash
go run github.com/tendermint/tendermint/cmd/tendermint@v0.35.0 init validator --home ./tendermint-home
```

Next, build the application:
```bash
go build -mod=mod -o my-app  # use -mod=mod to automatically update go.sum
```

Everything is now in place to run your application.

Run:
```bash
$ rm -rf /tmp/example
$ TMHOME="/tmp/example" tenderdash init validator

I[2019-07-16|18:40:36.480] Generated private validator                  module=main keyFile=/tmp/example/config/priv_validator_key.json stateFile=/tmp/example2/data/priv_validator_state.json
I[2019-07-16|18:40:36.481] Generated node key                           module=main path=/tmp/example/config/node_key.json
I[2019-07-16|18:40:36.482] Generated genesis file                       module=main path=/tmp/example/config/genesis.json
I[2019-07-16|18:40:36.483] Generated config                             module=main mode=validator
```

The application will begin producing blocks and you can see this reflected in
the log output.

You now have successfully started running an application using Tendermint Core 🎉🎉.

## 1.6 Using the application

Your application is now running and emitting logs to the terminal.
Now it's time to see what this application can do!

Let's try submitting a transaction to our new application.

Open another terminal window and run the following curl command:

```bash
$ curl -s 'localhost:26657/broadcast_tx_commit?tx="tendermint=rocks"'
{
  "check_tx": {
    "gasWanted": "1",
    ...
  },
  "deliver_tx": { ... },
  "hash": "1B3C5A1093DB952C331B1749A21DCCBB0F6C7F4E0055CD04D16346472FC60EC6",
  "height": "128"
}
```

If everything went well, you should see a response indicating which height the
transaction was included in the blockchain.

Finally, let's make sure that transaction really was persisted by the application.

Run the following command:

```bash
$ curl -s 'localhost:26657/abci_query?data="tendermint"'
{
  "response": {
    "code": 0,
    "log": "exists",
    "info": "",
    "index": "0",
    "key": "dGVuZGVybWludA==",
    "value": "cm9ja3M=",
    "proofOps": null,
    "height": "6",
    "codespace": ""
  }
}
```

Those values don't look like the `key` and `value` we sent to Tendermint,
what's going on here?

The response contain a `base64` encoded representation of the data we submitted.
To get the original value out of this data, we can use the `base64` command line utility.

Run:
```
echo cm9ja3M=" | base64 -d
```

## Outro

I hope everything went smoothly and your first, but hopefully not the last,
Tendermint Core application is up and running. If not, please [open an issue on
Github](https://github.com/dashevo/tenderdash/issues/new/choose). To dig
deeper, read [the docs](https://docs.tendermint.com/master/).
