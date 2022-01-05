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

## 1.1 Installing Go

Please refer to [the official guide for installing
Go](https://golang.org/doc/install).

Verify that you have the latest version of Go installed:

```bash
$ go version
go version go1.17.x darwin/amd64
```

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
BlockChain Interface (ABCI). All of the message types Tendermint uses for 
communicating with the application can be found in the ABCI [protobuf
file](https://github.com/tendermint/spec/blob/b695d30aae69933bc0e630da14949207d18ae02c/proto/tendermint/abci/types.proto).

We will begin by creating the basic scaffolding for an ABCI application in 
a new `app.go` file. The first step is to create a new type, `KVStoreApplication`
with the methods that implement the abci `Application` interface.

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

For this tutorial, we will use [badger](https://github.com/dgraph-io/badger). 
Badger is a fast embedded key-value store. 

First, add badger as a dependency of your go module using the `go get` command:

`go get github.com/dgraph-io/badger`

Next, let's update the application and its constructor to receive a handle to the 
database. 

Update the application struct as follows: 

```go
type KVStoreApplication struct {
	db           *badger.DB
	currentBatch *badger.Txn
}
```

And change the constructor to set the appropriate field when creating the application:

```go
func NewKVStoreApplication(db *badger.DB) *KVStoreApplication {
	return &KVStoreApplication{
		db: db,
	}
}
```

Don't worry about `currentBatch` for now, we'll get to that later.

### 1.3.1 CheckTx

When Tendermint Core receives a new transaction, Tendermint asks the application
if the transaction is acceptable. In our new application, let's implement some
basic validation for the transactions it will receive.

Add the following helper method to `app.go`:

```go
func (app *KVStoreApplication) validateTx(tx []byte) uint32 {
	parts := bytes.Split(tx, []byte("="))

	// first, check that the transaction is not malformed.
	if len(parts) != 2 {
		return 1
	}

	key, value := parts[0], parts[1]

	var txAlreadyExists bool

	// ok, we have a well-formed transaction, next let's check that we haven't
	// already stored the same transaction in the database before.
	dbErr := app.db.View(func(txn *badger.Txn) error {
		item, err := txn.Get(key)
		if err != nil {
			if err != badger.ErrKeyNotFound {
				return err
			}
			return nil
		}
		return item.Value(func(val []byte) error {
			if bytes.Equal(val, value) {
				txAlreadyExists = true
			}
			return nil
		})
	})

	// There was an error processing the transaction.
	// The application therefore cannot safely make progress so we crash.
	if dbErr != nil {
		panic(dbErr)
	}

	if txAlreadyExists {
		return 2
	}
	return 0

}
```

And call it from within your `CheckTx` method:

```go
func (app *KVStoreApplication) CheckTx(req abcitypes.RequestCheckTx) abcitypes.ResponseCheckTx {
	if code := app.validateTx(req.Tx); code != 0 {
		return abcitypes.ResponseCheckTx{Code: code}
	}
	return abcitypes.ResponseCheckTx{Code: 0}
}
```

Any response with a non-zero code will be considered invalid by Tendermint. 
Our `CheckTx` logic returns `0` to Tendermint when the transaction passes
the validation checks.

### 1.3.2 BeginBlock -> DeliverTx -> EndBlock -> Commit

When the Tendermint consensus engine has decided on the block, the block is transferred to the
application over three ABCI method calls: `BeginBlock`, `DeliverTx`, and `EndBlock`.

`BeginBlock` is called once to indicate to the application that it is about to
receive a block.

`DeliverTx` is called repeatedly, once for each `Tx` that was included in the block.

`EndBlock` is called once to indicate to the application that no more transactions
will be delivered to the application.

To implement these calls in our application we're going to make use of Badger's 
transaction mechanism. 

First, let's create a new Badger `Txn` during `BeginBlock`:

```go
func (app *KVStoreApplication) BeginBlock(req abcitypes.RequestBeginBlock) abcitypes.ResponseBeginBlock {
	app.currentBatch = app.db.NewTransaction(true)
	return abcitypes.ResponseBeginBlock{}
}
```

Next, let's modify `DeliverTx` to add the `key` and `value` to this `Txn` every time our application
receives a new `RequestDeliverTx`.

```go
func (app *KVStoreApplication) DeliverTx(req abcitypes.RequestDeliverTx) abcitypes.ResponseDeliverTx {
	if code := app.validateTx(req.Tx); code != 0 {
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
Note that we check the validity of the transaction _again_ during `DeliverTx`.
Transactions are not guaranteed to be valid when they are delivered to to an
application. 

Also note that we don't commit the Badger `Txn` we are building during `DeliverTx`. 
Other methods, such as `Query`, rely on a consistent view of the application's state.
The application should only update its state when the full block has been delivered.

The `Commit` method indicates that the full block has been delivered. During `Commit`,
the application should persist the pending `Txn`.

Let's modify our `Commit` method to persist the new state to the database:

```go
func (app *KVStoreApplication) Commit() abcitypes.ResponseCommit {
	app.currentBatch.Commit()
	return abcitypes.ResponseCommit{Data: []byte{}}
}
```


### 1.3.3 Query Method

We'll want to be able to determine if a transaction was committed to the state-machine.
To do this, let's implement the `Query` method in `app.go`:

```go
func (app *KVStoreApplication) Query(req abcitypes.RequestQuery) abcitypes.ResponseQuery {
	var resp abcitypes.ResponseQuery
	resp.Key = req.Data

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
		panic(dbErr)
	}
	return resp
}
```

## 1.3.4 Additional Methods

You'll noticed that we left several methods unchanged. Specifically, we have yet
to implement the `Info` and `InitChain` methods and we did not implement 
any of the `*Snapthot` methods. These methods are all important for running Tendermint
applications in production but are not required for getting a very simple application
up and running. 

To better understand these methods and why they are useful, check out the Tendermint 
[specification on ABCI]()

## 1.4 Starting an application and a Tendermint Core instance in the same process

Now that we have the basic functionality of our application in place, let's put it
all together inside of our `main.go` file.

Add the following code to your `main.go` file: 

```go
package main

import (
	"flag"
	"fmt"
	"os"
	"os/signal"
	"syscall"

	"github.com/dgraph-io/badger"
	"github.com/spf13/viper"
	abciclient "github.com/tendermint/tendermint/abci/client"
	cfg "github.com/tendermint/tendermint/config"
	"github.com/tendermint/tendermint/libs/log"
	nm "github.com/tendermint/tendermint/node"
	"github.com/tendermint/tendermint/types"
)

var homeDir string

func init() {
	flag.StringVar(&homeDir, "tm-home", "", "Path to the tendermint 'home' directory")
}

func main() {
	flag.Parse()
	if homeDir == "" {
		h, err := os.UserHomeDir()
		if err != nil {
			panic(err)
		}
		homeDir = fmt.Sprintf("%s/%s", h, ".tendermint")
	}
	config := cfg.DefaultValidatorConfig()

	config.SetRoot(homeDir)

	viper.SetConfigFile(fmt.Sprintf("%s/%s", homeDir, "config/config.toml"))
	if err := viper.ReadInConfig(); err != nil {
		panic(err)
	}
	if err := viper.Unmarshal(config); err != nil {
		panic(err)
	}
	if err := config.ValidateBasic(); err != nil {
		panic(err)
	}
	gf, err := types.GenesisDocFromFile(config.GenesisFile())
	if err != nil {
		panic(err)
	}

	db, err := badger.Open(badger.DefaultOptions("./badger").WithTruncate(true))
	if err != nil {
		panic(err)
	}
	defer db.Close()
	app := NewKVStoreApplication(db)
	acc := abciclient.NewLocalCreator(app)

	logger := log.MustNewDefaultLogger(log.LogFormatPlain, log.LogLevelInfo, false)
	node, err := nm.New(config, logger, acc, gf)
	if err != nil {
		panic(err)
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
	config := cfg.DefaultValidatorConfig()
	homeDir, err := os.UserHomeDir()
	if err != nil {
		panic(err)
	}
	rootDir := fmt.Sprintf("%s/%s", homeDir, ".tendermint")
	config.SetRoot(rootDir)

	viper.SetConfigFile(fmt.Sprintf("%s/%s", rootDir, "config/config.toml"))
	if err := viper.ReadInConfig(); err != nil {
		panic(err)
	}
	if err := viper.Unmarshal(config); err != nil {
		panic(err)
	}
	if err := config.ValidateBasic(); err != nil {
		panic(err)
	}
	gf, err := types.GenesisDocFromFile(config.GenesisFile())
	if err != nil {
		panic(err)
	}
...
```

Next, we create a database handle and use it to construct our ABCI application:

```go
...
	db, err := badger.Open(badger.DefaultOptions("./badger").WithTruncate(true))
	if err != nil {
		panic(err)
	}
	defer db.Close()
	app := NewKVStoreApplication(db)
	acc := abciclient.NewLocalCreator(app)
...
```

Then we construct a logger:
```go
...
	logger := log.MustNewDefaultLogger(log.LogFormatPlain, log.LogLevelInfo, false)
...
```

Now we have everything setup to run the Tendermint Core node. We construct
a node by passing it the configuration, the logger, a handle to our application and
the genesis file:

```go
...
	node, err := nm.New(config, logger, acc, gf)
	if err != nil {
		panic(err)
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
Let's instruct the `go` tooling to use the correct version of the Tendermint library.

From inside of the project directory, run:

```sh
go get github.com/tendermint/tendermint@v0.35.0
```

Next, we'll need to populate the Tendermint Core configuration files.
This command will create a `tendermint-home` directory in your project and add a basic set of configuration
files in `tendermint-home/config/`. For more information on what these files contain
see [the configuration documentation]()

From the root of your project, run:
```bash
go run github.com/tendermint/tendermint/cmd/tendermint@v0.35.0 init validator --home ./tendermint-home
```

Next, build the application: 
```bash
go build -mod=mod -o my-app
```

Everything is now in place to run your application.

Run: 
```bash
$ ./my-app -tm-home ./tendermint-home
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
```

If everything went well, you should see a response indicating which height the 
transaction was included in the blockchain.

Finally, let's make sure that transaction really was persisted by the application.

Run the following command:

```bash
$ curl -s 'localhost:26657/abci_query?data="tendermint"'
```

Let's examine the response object that this request returns.
The request returns a `json` object with a `key` and `value` field set.

```json
...
      "key": "dGVuZGVybWludA==",
      "value": "cm9ja3M=",
 ...
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
Github](https://github.com/tendermint/tendermint/issues/new/choose). To dig
deeper, read [the docs](https://docs.tendermint.com/master/).
