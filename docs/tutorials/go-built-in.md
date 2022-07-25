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

<<<<<<< HEAD
> Note: please use a released version of Tendermint with this guide. The guides will work with the latest released version.
> Be aware that they may not apply to unreleased changes on master.
> We strongly advise against using unreleased commits for your development.
=======
## Built-in app vs external app

Running your application inside the same process as Tendermint Core will give
you the best possible performance.

For other languages, your application have to communicate with Tendermint Core
through a TCP, Unix domain socket or gRPC.
>>>>>>> 6c302218e (Documentation: update go tutorials (#9048))

## 1.1 Installing Go

Please refer to [the official guide for installing
Go](https://go.dev/doc/install).

Verify that you have the latest version of Go installed:

```sh
$ go version
<<<<<<< HEAD
go version go1.17.5 darwin/amd64
=======
go version go1.18.x darwin/amd64
>>>>>>> 6c302218e (Documentation: update go tutorials (#9048))
```

Note that the exact patch number may differ as Go releases come out.
## 1.2 Creating a new Go project

We'll start by creating a new Go project. First, initialize the project folder with `go mod init`. Running this command should create the `go.mod` file.

```sh
$ mkdir kvstore
$ cd kvstore
$ go mod init github.com/<username>/kvstore
go: creating new go.mod: module github.com/<username>/kvstore
```

<<<<<<< HEAD
Inside the example directory create a `main.go` file with the following content:

> Note: there is no need to clone or fork Tendermint in this tutorial. 
=======
Inside the project directory, create a `main.go` file with the following content:
>>>>>>> 6c302218e (Documentation: update go tutorials (#9048))

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

<<<<<<< HEAD
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
=======
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

When a new transaction is added to the Tendermint Core, it will ask the application to check it (validate the format, signatures, etc.).

```go
import (
  "bytes"

  ...
)

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
>>>>>>> 6c302218e (Documentation: update go tutorials (#9048))
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

<<<<<<< HEAD
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

Finally, make sure to add the `bytes` package to the your import stanza
at the top of `app.go`:

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
=======
### 1.3.3 BeginBlock -> DeliverTx -> EndBlock -> Commit
>>>>>>> 6c302218e (Documentation: update go tutorials (#9048))

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

<<<<<<< HEAD
Finally, make sure to add the `log` library to the `import` stanza as well:

```go
import (
	"bytes"
	"log"

	"github.com/dgraph-io/badger/v3"
	abcitypes "github.com/tendermint/tendermint/abci/types"
)
```
=======
### 1.3.4 Query
>>>>>>> 6c302218e (Documentation: update go tutorials (#9048))

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

## 1.3.4 Additional Methods

You'll notice that we left several methods unchanged. Specifically, we have yet
to implement the `Info` and `InitChain` methods and we did not implement 
any of the `*Snapthot` methods. These methods are all important for running Tendermint
applications in production but are not required for getting a very simple application
up and running. 

To better understand these methods and why they are useful, check out the Tendermint 
[specification on ABCI](https://github.com/tendermint/spec/tree/20b2abb5f9a83c2d9d97b53e555e4ea5a6bd7dc4/spec/abci).

## 1.4 Starting an application and a Tendermint Core instance in the same process

Now that we have the basic functionality of our application in place, let's put it
all together inside of our `main.go` file.

Add the following code to your `main.go` file: 

```go
package main

import (
<<<<<<< HEAD
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
=======
 "flag"
 "fmt"
 "os"
 "os/signal"
 "path/filepath"
 "syscall"

 "github.com/dgraph-io/badger/v3"
 "github.com/spf13/viper"

  abciclient "github.com/tendermint/tendermint/abci/client"
  abcitypes "github.com/tendermint/tendermint/abci/types"
  tmconfig "github.com/tendermint/tendermint/config"
  tmlog "github.com/tendermint/tendermint/libs/log"
  tmservice "github.com/tendermint/tendermint/libs/service"
  tmnode "github.com/tendermint/tendermint/node"
>>>>>>> 6c302218e (Documentation: update go tutorials (#9048))
)

var homeDir string

func init() {
	flag.StringVar(&homeDir, "tm-home", "", "Path to the tendermint config directory (if empty, uses $HOME/.tendermint)")
}

func main() {
<<<<<<< HEAD
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
=======
 db, err := badger.Open(badger.DefaultOptions("/tmp/badger"))
 if err != nil {
  fmt.Fprintf(os.Stderr, "failed to open badger db: %v", err)
  os.Exit(1)
 }
 defer db.Close()
 app := NewKVStoreApplication(db)

 flag.Parse()

 node, err := newTendermint(app, configFile)
 if err != nil {
  fmt.Fprintf(os.Stderr, "%v", err)
  os.Exit(2)
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

func newTendermint(app abcitypes.Application, configFile string) (tmservice.Service, error) {
	// read config
	config := tmconfig.DefaultValidatorConfig()
	config.SetRoot(filepath.Dir(filepath.Dir(configFile)))

	viper.SetConfigFile(configFile)
	if err := viper.ReadInConfig(); err != nil {
		return nil, fmt.Errorf("viper failed to read config file: %w", err)
	}
	if err := viper.Unmarshal(config); err != nil {
		return nil, fmt.Errorf("viper failed to unmarshal config: %w", err)
	}
	if err := config.ValidateBasic(); err != nil {
		return nil, fmt.Errorf("config is invalid: %w", err)
	}

	// create logger
	logger, err := tmlog.NewDefaultLogger(tmlog.LogFormatPlain, config.LogLevel, false)
	if err != nil {
		return nil, fmt.Errorf("failed to create logger: %w", err)
	}

	// create node
	node, err := tmnode.New(
		config,
		logger,
		abciclient.NewLocalCreator(app),
		nil,
	)
	if err != nil {
		return nil, fmt.Errorf("failed to create new Tendermint node: %w", err)
	}

	return node, nil
}

```

This is a huge blob of code, so let's break it down into pieces.

First, we initialize the Badger database and create an app instance:

```go
db, err := badger.Open(badger.DefaultOptions("/tmp/badger"))
if err != nil {
 fmt.Fprintf(os.Stderr, "failed to open badger db: %v", err)
 os.Exit(1)
>>>>>>> 6c302218e (Documentation: update go tutorials (#9048))
}
```

This is a huge blob of code, so let's break down what it's doing.

<<<<<<< HEAD
First, we load in the Tendermint Core configuration files:
=======
```go
db, err := badger.Open(badger.DefaultOptions("/tmp/badger").WithTruncate(true))
```

Then we use it to create a Tendermint Core [Service](https://github.com/tendermint/tendermint/blob/v0.35.8/libs/service/service.go#L24) instance:
>>>>>>> 6c302218e (Documentation: update go tutorials (#9048))

```go
...
<<<<<<< HEAD
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
=======

// create node
node, err := tmnode.New(
  config,
  logger,
  abciclient.NewLocalCreator(app),
  nil,
)
if err != nil {
  return nil, fmt.Errorf("failed to create new Tendermint node: %w", err)
}
```

[tmnode.New](https://github.com/tendermint/tendermint/blob/v0.35.8/node/public.go#L29) requires a few things including a configuration file, a logger and a few others in order to construct the full node.

Note that we use [abciclient.NewLocalCreator](https://github.com/tendermint/tendermint/blob/v0.35.8/abci/client/creators.go#L15) here to create a local client instead of one communicating through a socket or gRPC.

[viper](https://github.com/spf13/viper) is being used for reading the config,
which we will generate later using the `tendermint init` command.

```go
// read config
config := tmconfig.DefaultValidatorConfig()
config.SetRoot(filepath.Dir(filepath.Dir(configFile)))
viper.SetConfigFile(configFile)
if err := viper.ReadInConfig(); err != nil {
  return nil, fmt.Errorf("viper failed to read config file: %w", err)
}
if err := viper.Unmarshal(config); err != nil {
  return nil, fmt.Errorf("viper failed to unmarshal config: %w", err)
}
if err := config.ValidateBasic(); err != nil {
  return nil, fmt.Errorf("config is invalid: %w", err)
}
```

As for the logger, we use the built-in library, which provides a nice
abstraction over [zerolog](https://github.com/rs/zerolog).

```go
// create logger
logger, err := tmlog.NewDefaultLogger(tmlog.LogFormatPlain, config.LogLevel, true)
if err != nil {
  return nil, fmt.Errorf("failed to create logger: %w", err)
}
>>>>>>> 6c302218e (Documentation: update go tutorials (#9048))
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

## 1.5 Getting up and running

<<<<<<< HEAD
Our application is almost ready to run.
Let's install the latest release version of the Tendermint library.

From inside of the project directory, run:

```sh
go get github.com/tendermint/tendermint@v0.35.0
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
=======
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
>>>>>>> 6c302218e (Documentation: update go tutorials (#9048))
```

Everything is now in place to run your application.

<<<<<<< HEAD
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
=======
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
$ ./kvstore -config "/tmp/kvstore/config/config.toml"

badger 2022/07/16 13:55:59 INFO: All 0 tables opened in 0s
badger 2022/07/16 13:55:59 INFO: Replaying file id: 0 at offset: 0
badger 2022/07/16 13:55:59 INFO: Replay took: 3.052µs
badger 2022/07/16 13:55:59 DEBUG: Value log discard stats empty
2022-07-16T13:55:59+08:00 INFO starting service impl=multiAppConn module=proxy service=multiAppConn
2022-07-16T13:55:59+08:00 INFO starting service connection=query impl=localClient module=abci-client service=localClient
2022-07-16T13:55:59+08:00 INFO starting service connection=snapshot impl=localClient module=abci-client service=localClient
2022-07-16T13:55:59+08:00 INFO starting service connection=mempool impl=localClient module=abci-client service=localClient
2022-07-16T13:55:59+08:00 INFO starting service connection=consensus impl=localClient module=abci-client service=localClient
2022-07-16T13:55:59+08:00 INFO starting service impl=EventBus module=events service=EventBus
2022-07-16T13:55:59+08:00 INFO starting service impl=PubSub module=pubsub service=PubSub
2022-07-16T13:55:59+08:00 INFO starting service impl=IndexerService module=txindex service=IndexerService
2022-07-16T13:55:59+08:00 INFO ABCI Handshake App Info hash= height=0 module=consensus protocol-version=0 software-version=
2022-07-16T13:55:59+08:00 INFO ABCI Replay Blocks appHeight=0 module=consensus stateHeight=0 storeHeight=0
2022-07-16T13:55:59+08:00 INFO Completed ABCI Handshake - Tendermint and App are synced appHash= appHeight=0 module=consensus
2022-07-16T13:55:59+08:00 INFO Version info block=11 mode=validator p2p=8 tmVersion=0.35.8
```

Let's try sending a transaction. Open another terminal and execute the below command.
>>>>>>> 6c302218e (Documentation: update go tutorials (#9048))

```sh
$ curl -s 'localhost:26657/broadcast_tx_commit?tx="tendermint=rocks"'
<<<<<<< HEAD
=======
{
  ...
  "result": {
    "check_tx": {
      ...
      "gas_wanted": "1",
      ...
    },
    "deliver_tx": {...},
    "hash": "1B3C5A1093DB952C331B1749A21DCCBB0F6C7F4E0055CD04D16346472FC60EC6",
    "height": "91"
  }
}
>>>>>>> 6c302218e (Documentation: update go tutorials (#9048))
```

If everything went well, you should see a response indicating which height the 
transaction was included in the blockchain.

<<<<<<< HEAD
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
=======
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
>>>>>>> 6c302218e (Documentation: update go tutorials (#9048))

## Outro

I hope everything went smoothly and your first, but hopefully not the last,
Tendermint Core application is up and running. If not, please [open an issue on
Github](https://github.com/tendermint/tendermint/issues/new/choose). To dig
deeper, read [the docs](https://docs.tendermint.com/master/).
