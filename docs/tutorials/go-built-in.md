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

The application will be written in Go and use Tendermint as a library and  
some understanding of the Go programming language is expected.
If you have never written Go, you may want to go through [Learn X in Y minutes
Where X=Go](https://learnxinyminutes.com/docs/go/) first, to familiarize
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
dependency management, so let's start by including a dependency on the latest
Tendermint.

```bash
go mod init kvstore
go get github.com/tendermint/tendermint/@latest
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
got get
go build
```


## 1.3 Writing a Tendermint Core application

Tendermint Core communicates with the application through the Application
BlockChain Interface (ABCI). The messages exchanged through the interface are 
defined in the ABCI [protobuf
file](https://github.com/tendermint/tendermint/blob/main/proto/tendermint/abci/types.proto).

We begin by creating the basic scaffolding for an ABCI application by 
creating a in a new type, `KVStoreApplication`, which implements the
methods defined by the `abcitypes.Application` interface.

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

func (app KVStoreApplication) ProcessProposal(proposal abcitypes.RequestProcessProposal) abcitypes.ResponseProcessProposal {
	return abcitypes.ResponseProcessProposal{Status: 1}
}

```

The types used here are defined in the Tendermint library and were added as a dependency
to the project when you ran `go get`.

```bash
go get github.com/tendermint/tendermint@latest
```

You can recompile the application now, but it isn't very useful.
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

var _ abcitypes.Application = (*KVStoreApplication)(nil)

func NewKVStoreApplication(db *badger.DB) *KVStoreApplication {
	return &KVStoreApplication{db: db}
}
```

The pendingBlock keeps track of the transactions that will update the application's state when a block 
is completed. Don't worry about it for now, we'll get to that later.

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
- `DeliverTx` is called repeatedly, once for each application transaction, `Tx` that was included in the block.
- `EndBlock` is called once to indicate to the application that no more transactions
will be delivered to the application.

:Note: To implement these calls in our application we're going to make use of Badger's 
transaction mechanism. We will always refer to these as Badger transactions, not to
confuse them with the transactions included in the blocks delieverd by Tendermint,
the _blockchain transactions_.

First, let's create a new Badger transaction during `BeginBlock` and return informing Tendermint
that the application is ready to receive application transactions:


```go
func (app *KVStoreApplication) BeginBlock(req abcitypes.RequestBeginBlock) abcitypes.ResponseBeginBlock {
	app.pendingBlock = app.db.NewTransaction(true)
	return abcitypes.ResponseBeginBlock{}
}
```

Next, let's modify `DeliverTx` to add the `key` and `value` to the database transaction every time our application
receives a new transaction through `RequestDeliverTx`.

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
validity. Application state may have changed between the initial execution of `CheckTx`
and the transaction delivery in `DeliverTx` in a way that rendered the transaction
no longer valid.

Also note that we **cannot** commit the Badger transaction we are building during `DeliverTx`.
Other methods, such as `Query`, rely on a consistent view of the application's state.
The application should only update it state by committing the Badger transactions 
when the full block has been delivered, in the `Commit` method.

The `Commit` method tells the application that the full block has been delivered. 
Let's update the method to terminate the pending Badger transaction and
persist the resulting state: 

```go
func (app *KVStoreApplication) Commit() abcitypes.ResponseCommit {
	if err := app.pendingBlock.Commit(); err != nil {
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
an unexpected error from the database during the `DeliverTx` or `Commit` methods. This
is not an accident. If the application received an error from the database, there 
is no deterministic way for it to make progress so the only safe option is to terminate.

### 1.3.4 Query

We'll want to read key/value pairs written to `kvstore` application.
To do this, let's rewrite the Query method in `app.go`:

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


## 1.4 Starting an application and a Tendermint Core instance in the same process

Now that we have the basic functionality of our application in place, let's put it all together inside of our main.go file.

Add the following code to your `main.go` file:


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
Now we have everything setup to run the Tendermint node. We construct
a node by passing it the configuration, the logger, a handle to our application and
the genesis information:

```go
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
```

Finally, we start the node, i.e., the Tendermint Core service inside our application:

```go
	node.Start()
	defer func() {
		node.Stop()
		node.Wait()
	}()
```
The additional logic at the end of the file allows the program to catch SIGTERM. This means that the node can shutdown gracefully when an operator tries to kill the program:

```go
	c := make(chan os.Signal, 1)
	signal.Notify(c, os.Interrupt, syscall.SIGTERM)
	<-c
```

## 1.5 Getting Up and Running

Our application is almost ready to run, but first we'll need to populate the Tendermint Core configuration files.
The following command will create a `tendermint-home` directory in your project and add a basic set of configuration files in `tendermint-home/config/`.
For more information on what these files contain see [the configuration documentation](https://github.com/tendermint/tendermint/blob/v0.37.0/docs/nodes/configuration.md).

From the root of your project, run:

```bash
go run github.com/tendermint/tendermint/cmd/tendermint@v0.37.0-rc1 init --home /tmp/tendermint-home
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
./kvstore -tm-home /tmp/tendermint-home
```

The application will start with and you should see a continuous output starting with:

```bash
badger 2022/11/09 09:08:50 INFO: All 0 tables opened in 0s
badger 2022/11/09 09:08:50 INFO: Discard stats nextEmptySlot: 0
badger 2022/11/09 09:08:50 INFO: Set nextTxnTs to 0
I[2022-11-09|09:08:50.085] service start                                module=proxy msg="Starting multiAppConn service" impl=multiAppConn
I[2022-11-09|09:08:50.085] service start                                module=abci-client connection=query msg="Starting localClient service" impl=localClient
I[2022-11-09|09:08:50.085] service start                                module=abci-client connection=snapshot msg="Starting localClient service" impl=localClient
...
```

More importantly, the application using Tendermint Core is producing blocks  🎉🎉 and you can see this reflected in the log output in lines like this:

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
