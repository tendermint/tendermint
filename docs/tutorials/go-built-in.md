<!---
order: 2
--->

# Creating a built-in application in Go

## Guide assumptions

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

The application will be written in Go and use Tendermint as a library. Hence,  
some understanding of the Go programming language is expected.
If you have never written Go, you may want to go through [Learn X in Y minutes
Where X=Go](https://learnxinyminutes.com/docs/go/) first to familiarize
yourself with the syntax.

Note: Please use the latest released version of this guide and of Tendermint.
We strongly advise against using unreleased commits for your development.


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
cd kvstore
go mod init kvstore
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
$ go run main.go
Hello, Tendermint Core
```

## 1.3 Writing a Tendermint Core application

Tendermint Core communicates with the application through the Application
BlockChain Interface (ABCI). The messages exchanged through the interface are 
defined in the ABCI [protobuf
file](https://github.com/tendermint/tendermint/blob/main/proto/tendermint/abci/types.proto).

We begin by creating the basic scaffolding for an ABCI application by 
creating a in a new type, `KVStoreApplication`, with methods that implement 
the ABCI `Application` interface.

Create a file called `app.go` with the following contents:

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

These are the structure and methods that built-in Tendermint Core application must implement.
They are defined in the Tendermint library and are imported using 'go get'.

```bash
go get github.com/tendermint/tendermint@latest
```

You can compile the application now, but it isn't very useful.
So let's revisit the code adding the logic needed to implement our minimal key/value store.


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
	pendingBlock *badger.Txn
}
```

And change the constructor to set the appropriate field when creating the application:

```go
func NewKVStoreApplication(db *badger.DB) *KVStoreApplication {
	return &KVStoreApplication{db: db}
}
```

The pendingBlock keeps track of the transactions that will update the application's state when a block is completed. Don't worry about it for now, we'll get to that later.

Finally, update the import stanza at the top to include the Badger library:

```go
import(
	"github.com/dgraph-io/badger/v3"
	abcitypes "github.com/tendermint/tendermint/abci/types"
)
```

### 1.3.2 CheckTx


When Tendermint Core receives a new transaction, Tendermint asks the application if the transaction is acceptable. 

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

While this CheckTx is simple and only validates that the transaction is well-formed, 
it is very common for `CheckTx` to make more complex use of the state of an application.
For example, you may refuse to overwrite an existing value, or you can associate 
versions to the key/value pairs and allow the caller to specify a version to
perform a conditional update.

Depending on the checks and on the conditions violated, the function may return
different values, but any response with a non-zero code will be considered invalid 
by Tendermint. Our CheckTx logic returns 0 to Tendermint when a transaction passes 
its validation checks. The specific value of the code is meaningless to Tendermint. 
Non-zero codes are logged by Tendermint so applications can provide more specific 
information on why the transaction was rejected.

Note that CheckTx does not execute the transaction, it only verifies that that the transaction could be executed. We do not know yet if the rest of the network has agreed to accept this transaction into a block.


Finally, make sure to add the bytes package to the your import stanza at the top of app.go:

```go
import(
	"bytes"

	"github.com/dgraph-io/badger/v3"
	abcitypes "github.com/tendermint/tendermint/abci/types"
)
```


### 1.3.2 BeginBlock -> DeliverTx -> EndBlock -> Commit

When the Tendermint consensus engine has decided on the block, the block is transferred to the
application over three ABCI method calls: `BeginBlock`, `DeliverTx`, and `EndBlock`.

- `BeginBlock` is called once to indicate to the application that it is about to
receive a block.
- `DeliverTx` is called repeatedly, once for each `Tx` that was included in the block.
- `EndBlock` is called once to indicate to the application that no more transactions
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
	if code := app.isValid(req.Tx); code != 0 {
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
application, even if they were valid when they were propoposed. 
This can happen if the application state is used to determine transaction
validity. Application state may have changed between when the `CheckTx` was initially
called and when the transaction was delivered in `DeliverTx` in a way that rendered
the transaction no longer valid.

Also note that we **cannot** commit the Badger `Txn` we are building during `DeliverTx`.
Other methods, such as `Query`, rely on a consistent view of the application's state.
The application should only update its state when the full block has been delivered.

The Commit method indicates that the full block has been delivered. During Commit,
the application should persist the pending Txn.

Let's modify our Commit method to persist the new state to the database:

```go
func (app *KVStoreApplication) Commit() abcitypes.ResponseCommit {
	if err := app.pendingBlock.Commit(); err != nil {
		log.Panicf("Error writing to database, unable to commit block: %v", err)
	}
	return abcitypes.ResponseCommit{Data: []byte{}}
}
```

Finally, make sure to add the log library to the import stanza as well:

```go
import (
	"bytes"
	"log"

	"github.com/dgraph-io/badger/v3"
	abcitypes "github.com/tendermint/tendermint/abci/types"
)
```

You may have noticed that the application we are writing will crash if it receives
an unexpected error from the database during the DeliverTx or Commit methods. This
is not an accident. If the application received an error from the database, there 
is no deterministic way for it to make progress so the only safe option is to terminate.

### 1.3.4 Query


We'll want to be able to determine if a transaction was committed to the state-machine. 
To do this, let's implement the Query method in app.go:

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


### 1.3.5 Additional Methods

You'll notice that we left several methods unchanged. Specifically, we have yet to 
implement the `Info` and `InitChain` methods and we did not implement any of the
`*Snapthot` methods. These methods are all important for running Tendermint 
applications in production but are not required for getting a very simple 
application up and running. To better understand these methods and why they are 
useful, check out the Tendermint specification on 
[ABCI](https://github.com/tendermint/tendermint/tree/main/spec/abci/).




## 1.4 Starting an application and a Tendermint Core instance in the same process

Now that we have the basic functionality of our application in place, let's put it all together inside of our main.go file.

Add the following code to your main.go file:


```go
package main

import (
	"flag"
	"fmt"
	"github.com/tendermint/tendermint/p2p"
	"github.com/tendermint/tendermint/privval"
	"github.com/tendermint/tendermint/proxy"
	"log"
	"os"
	"os/signal"
	"path/filepath"
	"syscall"

	"github.com/dgraph-io/badger/v3"
	"github.com/spf13/viper"
	cfg "github.com/tendermint/tendermint/config"
	tmflags "github.com/tendermint/tendermint/libs/cli/flags"
	tmlog "github.com/tendermint/tendermint/libs/log"
	nm "github.com/tendermint/tendermint/node"
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
	config := cfg.DefaultConfig()

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

	pv := privval.LoadFilePV(
		config.PrivValidatorKeyFile(),
		config.PrivValidatorStateFile(),
	)

	// read node key
	nodeKey, err := p2p.LoadNodeKey(config.NodeKeyFile())
	if err != nil {
		log.Fatalf("failed to load node's key: %v", err)
	}

	logger := tmlog.NewTMLogger(tmlog.NewSyncWriter(os.Stdout))
	logger, err = tmflags.ParseLogLevel(config.LogLevel, logger, cfg.DefaultLogLevel)
	if err != nil {
		log.Fatalf("failed to parse log level: %v", err)
	}
	node, err := nm.NewNode(
		config,
		pv,
		nodeKey,
		proxy.NewLocalClientCreator(app),
		nm.DefaultGenesisDocProviderFunc(config),
		nm.DefaultDBProvider,
		nm.DefaultMetricsProvider(config.Instrumentation),
		logger)

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

This is a huge blob of code, so let's break it down into pieces.

First, we use [viper](https://github.com/spf13/viper) to load the Tendermint Core configuration files, which we will generate later using the tendermint init command:


```go
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
```


We use `FilePV`, which is a private validator (i.e. thing which signs consensus
messages). Normally, you would use `SignerRemote` to connect to an external
[HSM](https://kb.certus.one/hsm.html).

```go
	pv := privval.LoadFilePV(
		config.PrivValidatorKeyFile(),
		config.PrivValidatorStateFile(),
	)
```

`nodeKey` is needed to identify the node in a p2p network.

```go
	nodeKey, err := p2p.LoadNodeKey(config.NodeKeyFile())
	if err != nil {
		return nil, fmt.Errorf("failed to load node's key: %w", err)
	}
```


## 1.5 Getting Up and Running

We are going to use [Go modules](https://github.com/golang/go/wiki/Modules) for
dependency management.

```bash
go mod init github.com/me/example
go get github.com/tendermint/tendermint/@v0.34.0
```

After running the above commands you will see two generated files, go.mod and go.sum. The go.mod file should look similar to:

```go
module github.com/me/example

go 1.15

require (
	github.com/dgraph-io/badger v1.6.2
	github.com/tendermint/tendermint v0.34.0
)
```

Finally, we will build our binary:

```sh
go build
```

To create a default configuration, nodeKey and private validator files, let's
execute `tendermint init`. But before we do that, we will need to install
Tendermint Core. Please refer to [the official
guide](https://docs.tendermint.com/main/introduction/install.html). If you're
installing from source, don't forget to checkout the latest release (`git checkout vX.Y.Z`).

```bash
$ rm -rf /tmp/example
$ TMHOME="/tmp/example" tendermint init

I[2019-07-16|18:40:36.480] Generated private validator                  module=main keyFile=/tmp/example/config/priv_validator_key.json stateFile=/tmp/example2/data/priv_validator_state.json
I[2019-07-16|18:40:36.481] Generated node key                           module=main path=/tmp/example/config/node_key.json
I[2019-07-16|18:40:36.482] Generated genesis file                       module=main path=/tmp/example/config/genesis.json
```

We are ready to start our application:

```bash
$ ./example -config "/tmp/example/config/config.toml"

badger 2019/07/16 18:42:25 INFO: All 0 tables opened in 0s
badger 2019/07/16 18:42:25 INFO: Replaying file id: 0 at offset: 0
badger 2019/07/16 18:42:25 INFO: Replay took: 695.227s
E[2019-07-16|18:42:25.818] Couldn't connect to any seeds                module=p2p
I[2019-07-16|18:42:26.853] Executed block                               module=state height=1 validTxs=0 invalidTxs=0
I[2019-07-16|18:42:26.865] Committed state                              module=state height=1 txs=0 appHash=
```

Now open another tab in your terminal and try sending a transaction:

```bash
$ curl -s 'localhost:26657/broadcast_tx_commit?tx="tendermint=rocks"'
{
  "jsonrpc": "2.0",
  "id": "",
  "result": {
    "check_tx": {
      "gasWanted": "1"
    },
    "deliver_tx": {},
    "hash": "1B3C5A1093DB952C331B1749A21DCCBB0F6C7F4E0055CD04D16346472FC60EC6",
    "height": "128"
  }
}
```

Response should contain the height where this transaction was committed.

Now let's check if the given key now exists and its value:

```json
$ curl -s 'localhost:26657/abci_query?data="tendermint"'
{
  "jsonrpc": "2.0",
  "id": "",
  "result": {
    "response": {
      "log": "exists",
      "key": "dGVuZGVybWludA==",
      "value": "cm9ja3M="
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
deeper, read [the docs](https://docs.tendermint.com/main/).
