<!---
order: 4
--->

# Creating an application in Kotlin

## Guide Assumptions

This guide is designed for beginners who want to get started with a Tendermint
Core application from scratch. It does not assume that you have any prior
experience with Tendermint Core.

Tendermint Core is Byzantine Fault Tolerant (BFT) middleware that takes a state
transition machine (your application) - written in any programming language - and securely
replicates it on many machines.

By following along with this guide, you'll create a Tendermint Core project
called kvstore, a (very) simple distributed BFT key-value store. The application (which should
implementing the blockchain interface (ABCI)) will be written in Kotlin.

This guide assumes that you are not new to JVM world. If you are new please see [JVM Minimal Survival Guide](https://hadihariri.com/2013/12/29/jvm-minimal-survival-guide-for-the-dotnet-developer/#java-the-language-java-the-ecosystem-java-the-jvm) and [Gradle Docs](https://docs.gradle.org/current/userguide/userguide.html).

## Built-in app vs external app

If you use Golang, you can run your app and Tendermint Core in the same process to get maximum performance.
[Cosmos SDK](https://github.com/cosmos/cosmos-sdk) is written this way.
Please refer to [Writing a built-in Tendermint Core application in Go](./go-built-in.md) guide for details.

If you choose another language, like we did in this guide, you have to write a separate app,
which will communicate with Tendermint Core via a socket (UNIX or TCP) or gRPC.
This guide will show you how to build external application using RPC server.

Having a separate application might give you better security guarantees as two
processes would be communicating via established binary protocol. Tendermint
Core will not have access to application's state.

## 1.1 Installing Java and Gradle

Please refer to [the Oracle's guide for installing JDK](https://www.oracle.com/technetwork/java/javase/downloads/index.html).

Verify that you have installed Java successfully:

```sh
$ java -version
java version "1.8.0_162"
Java(TM) SE Runtime Environment (build 1.8.0_162-b12)
Java HotSpot(TM) 64-Bit Server VM (build 25.162-b12, mixed mode)
```

You can choose any version of Java higher or equal to 8.
In my case it is Java SE Development Kit 8.

Make sure you have `$JAVA_HOME` environment variable set:

```sh
$ echo $JAVA_HOME
/Library/Java/JavaVirtualMachines/jdk1.8.0_162.jdk/Contents/Home
```

For Gradle installation, please refer to [their official guide](https://gradle.org/install/).

## 1.2 Creating a new Kotlin project

We'll start by creating a new Gradle project.

```sh
$ export KVSTORE_HOME=~/kvstore
$ mkdir $KVSTORE_HOME
$ cd $KVSTORE_HOME
```

Inside the example directory run:

```sh
gradle init --dsl groovy --package io.example --project-name example --type kotlin-application
```

This will create a new project for you. The tree of files should look like:

```sh
$ tree
.
|-- build.gradle
|-- gradle
|   `-- wrapper
|       |-- gradle-wrapper.jar
|       `-- gradle-wrapper.properties
|-- gradlew
|-- gradlew.bat
|-- settings.gradle
`-- src
    |-- main
    |   |-- kotlin
    |   |   `-- io
    |   |       `-- example
    |   |           `-- App.kt
    |   `-- resources
    `-- test
        |-- kotlin
        |   `-- io
        |       `-- example
        |           `-- AppTest.kt
        `-- resources
```

When run, this should print "Hello world." to the standard output.

```sh
$ ./gradlew run
> Task :run
Hello world.
```

## 1.3 Writing a Tendermint Core application

Tendermint Core communicates with the application through the Application
BlockChain Interface (ABCI). All message types are defined in the [protobuf
file](https://github.com/tendermint/tendermint/blob/master/abci/types/types.proto).
This allows Tendermint Core to run applications written in any programming
language.

### 1.3.1 Compile .proto files

Add the following piece to the top of the `build.gradle`:

```groovy
buildscript {
    repositories {
        mavenCentral()
    }
    dependencies {
        classpath 'com.google.protobuf:protobuf-gradle-plugin:0.8.8'
    }
}
```

Enable the protobuf plugin in the `plugins` section of the `build.gradle`:

```groovy
plugins {
    id 'com.google.protobuf' version '0.8.8'
}
```

Add the following code to `build.gradle`:

```groovy
protobuf {
    protoc {
        artifact = "com.google.protobuf:protoc:3.7.1"
    }
    plugins {
        grpc {
            artifact = 'io.grpc:protoc-gen-grpc-java:1.22.1'
        }
    }
    generateProtoTasks {
        all()*.plugins {
            grpc {}
        }
    }
}
```

Now we should be ready to compile the `*.proto` files.

Copy the necessary `.proto` files to your project:

```sh
mkdir -p \
  $KVSTORE_HOME/src/main/proto/github.com/tendermint/tendermint/abci/types \
  $KVSTORE_HOME/src/main/proto/github.com/tendermint/tendermint/crypto/merkle \
  $KVSTORE_HOME/src/main/proto/github.com/tendermint/tendermint/libs/kv \
  $KVSTORE_HOME/src/main/proto/github.com/gogo/protobuf/gogoproto

cp $GOPATH/src/github.com/tendermint/tendermint/abci/types/types.proto \
   $KVSTORE_HOME/src/main/proto/github.com/tendermint/tendermint/abci/types/types.proto
cp $GOPATH/src/github.com/tendermint/tendermint/crypto/merkle/merkle.proto \
   $KVSTORE_HOME/src/main/proto/github.com/tendermint/tendermint/crypto/merkle/merkle.proto
cp $GOPATH/src/github.com/tendermint/tendermint/libs/kv/types.proto \
   $KVSTORE_HOME/src/main/proto/github.com/tendermint/tendermint/libs/kv/types.proto
cp $GOPATH/src/github.com/gogo/protobuf/gogoproto/gogo.proto \
   $KVSTORE_HOME/src/main/proto/github.com/gogo/protobuf/gogoproto/gogo.proto
```

Add these dependencies to `build.gradle`:

```groovy
dependencies {
    implementation 'io.grpc:grpc-protobuf:1.22.1'
    implementation 'io.grpc:grpc-netty-shaded:1.22.1'
    implementation 'io.grpc:grpc-stub:1.22.1'
}
```

To generate all protobuf-type classes run:

```sh
./gradlew generateProto
```

To verify that everything went smoothly, you can inspect the `build/generated/` directory:

```sh
$ tree build/generated/
build/generated/
`-- source
    `-- proto
        `-- main
            |-- grpc
            |   `-- types
            |       `-- ABCIApplicationGrpc.java
            `-- java
                |-- com
                |   `-- google
                |       `-- protobuf
                |           `-- GoGoProtos.java
                |-- common
                |   `-- Types.java
                |-- merkle
                |   `-- Merkle.java
                `-- types
                    `-- Types.java
```

### 1.3.2 Implementing ABCI

The resulting `$KVSTORE_HOME/build/generated/source/proto/main/grpc/types/ABCIApplicationGrpc.java` file
contains the abstract class `ABCIApplicationImplBase`, which is an interface we'll need to implement.

Create `$KVSTORE_HOME/src/main/kotlin/io/example/KVStoreApp.kt` file with the following content:

```kotlin
package io.example

import io.grpc.stub.StreamObserver
import types.ABCIApplicationGrpc
import types.Types.*

class KVStoreApp : ABCIApplicationGrpc.ABCIApplicationImplBase() {

    // methods implementation

}
```

Now I will go through each method of `ABCIApplicationImplBase` explaining when it's called and adding
required business logic.

### 1.3.3 CheckTx

When a new transaction is added to the Tendermint Core, it will ask the
application to check it (validate the format, signatures, etc.).

```kotlin
override fun checkTx(req: RequestCheckTx, responseObserver: StreamObserver<ResponseCheckTx>) {
    val code = req.tx.validate()
    val resp = ResponseCheckTx.newBuilder()
            .setCode(code)
            .setGasWanted(1)
            .build()
    responseObserver.onNext(resp)
    responseObserver.onCompleted()
}

private fun ByteString.validate(): Int {
    val parts = this.split('=')
    if (parts.size != 2) {
        return 1
    }
    val key = parts[0]
    val value = parts[1]

    // check if the same key=value already exists
    val stored = getPersistedValue(key)
    if (stored != null && stored.contentEquals(value)) {
        return 2
    }

    return 0
}

private fun ByteString.split(separator: Char): List<ByteArray> {
    val arr = this.toByteArray()
    val i = (0 until this.size()).firstOrNull { arr[it] == separator.toByte() }
            ?: return emptyList()
    return listOf(
            this.substring(0, i).toByteArray(),
            this.substring(i + 1).toByteArray()
    )
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
specification"](https://docs.tendermint.com/master/spec/abci/apps.html#gas).

For the underlying key-value store we'll use
[JetBrains Xodus](https://github.com/JetBrains/xodus), which is a transactional schema-less embedded high-performance database written in Java.

`build.gradle`:

```groovy
dependencies {
    implementation 'org.jetbrains.xodus:xodus-environment:1.3.91'
}
```

```kotlin
...
import jetbrains.exodus.ArrayByteIterable
import jetbrains.exodus.env.Environment
import jetbrains.exodus.env.Store
import jetbrains.exodus.env.StoreConfig
import jetbrains.exodus.env.Transaction

class KVStoreApp(
    private val env: Environment
) : ABCIApplicationGrpc.ABCIApplicationImplBase() {

    private var txn: Transaction? = null
    private var store: Store? = null

    ...

    private fun getPersistedValue(k: ByteArray): ByteArray? {
        return env.computeInReadonlyTransaction { txn ->
            val store = env.openStore("store", StoreConfig.WITHOUT_DUPLICATES, txn)
            store.get(txn, ArrayByteIterable(k))?.bytesUnsafe
        }
    }
}
```

### 1.3.4 BeginBlock -> DeliverTx -> EndBlock -> Commit

When Tendermint Core has decided on the block, it's transferred to the
application in 3 parts: `BeginBlock`, one `DeliverTx` per transaction and
`EndBlock` in the end. `DeliverTx` are being transferred asynchronously, but the
responses are expected to come in order.

```kotlin
override fun beginBlock(req: RequestBeginBlock, responseObserver: StreamObserver<ResponseBeginBlock>) {
    txn = env.beginTransaction()
    store = env.openStore("store", StoreConfig.WITHOUT_DUPLICATES, txn!!)
    val resp = ResponseBeginBlock.newBuilder().build()
    responseObserver.onNext(resp)
    responseObserver.onCompleted()
}
```

Here we begin a new transaction, which will accumulate the block's transactions and open the corresponding store.

```kotlin
override fun deliverTx(req: RequestDeliverTx, responseObserver: StreamObserver<ResponseDeliverTx>) {
    val code = req.tx.validate()
    if (code == 0) {
        val parts = req.tx.split('=')
        val key = ArrayByteIterable(parts[0])
        val value = ArrayByteIterable(parts[1])
        store!!.put(txn!!, key, value)
    }
    val resp = ResponseDeliverTx.newBuilder()
            .setCode(code)
            .build()
    responseObserver.onNext(resp)
    responseObserver.onCompleted()
}
```

If the transaction is badly formatted or the same key=value already exist, we
again return the non-zero code. Otherwise, we add it to the store.

In the current design, a block can include incorrect transactions (those who
passed `CheckTx`, but failed `DeliverTx` or transactions included by the proposer
directly). This is done for performance reasons.

Note we can't commit transactions inside the `DeliverTx` because in such case
`Query`, which may be called in parallel, will return inconsistent data (i.e.
it will report that some value already exist even when the actual block was not
yet committed).

`Commit` instructs the application to persist the new state.

```kotlin
override fun commit(req: RequestCommit, responseObserver: StreamObserver<ResponseCommit>) {
    txn!!.commit()
    val resp = ResponseCommit.newBuilder()
            .setData(ByteString.copyFrom(ByteArray(8)))
            .build()
    responseObserver.onNext(resp)
    responseObserver.onCompleted()
}
```

### 1.3.5 Query

Now, when the client wants to know whenever a particular key/value exist, it
will call Tendermint Core RPC `/abci_query` endpoint, which in turn will call
the application's `Query` method.

Applications are free to provide their own APIs. But by using Tendermint Core
as a proxy, clients (including [light client
package](https://godoc.org/github.com/tendermint/tendermint/lite)) can leverage
the unified API across different applications. Plus they won't have to call the
otherwise separate Tendermint Core API for additional proofs.

Note we don't include a proof here.

```kotlin
override fun query(req: RequestQuery, responseObserver: StreamObserver<ResponseQuery>) {
    val k = req.data.toByteArray()
    val v = getPersistedValue(k)
    val builder = ResponseQuery.newBuilder()
    if (v == null) {
        builder.log = "does not exist"
    } else {
        builder.log = "exists"
        builder.key = ByteString.copyFrom(k)
        builder.value = ByteString.copyFrom(v)
    }
    responseObserver.onNext(builder.build())
    responseObserver.onCompleted()
}
```

The complete specification can be found
[here](https://docs.tendermint.com/master/spec/abci/).

## 1.4 Starting an application and a Tendermint Core instances

Put the following code into the `$KVSTORE_HOME/src/main/kotlin/io/example/App.kt` file:

```kotlin
package io.example

import jetbrains.exodus.env.Environments

fun main() {
    Environments.newInstance("tmp/storage").use { env ->
        val app = KVStoreApp(env)
        val server = GrpcServer(app, 26658)
        server.start()
        server.blockUntilShutdown()
    }
}
```

It is the entry point of the application.
Here we create a special object `Environment`, which knows where to store the application state.
Then we create and start the gRPC server to handle Tendermint Core requests.

Create `$KVSTORE_HOME/src/main/kotlin/io/example/GrpcServer.kt` file with the following content:

```kotlin
package io.example

import io.grpc.BindableService
import io.grpc.ServerBuilder

class GrpcServer(
        private val service: BindableService,
        private val port: Int
) {
    private val server = ServerBuilder
            .forPort(port)
            .addService(service)
            .build()

    fun start() {
        server.start()
        println("gRPC server started, listening on $port")
        Runtime.getRuntime().addShutdownHook(object : Thread() {
            override fun run() {
                println("shutting down gRPC server since JVM is shutting down")
                this@GrpcServer.stop()
                println("server shut down")
            }
        })
    }

    fun stop() {
        server.shutdown()
    }

    /**
     * Await termination on the main thread since the grpc library uses daemon threads.
     */
    fun blockUntilShutdown() {
        server.awaitTermination()
    }

}
```

## 1.5 Getting Up and Running

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
[here](https://docs.tendermint.com/master/tendermint-core/configuration.html).

We are ready to start our application:

```sh
./gradlew run

gRPC server started, listening on 26658
```

Then we need to start Tendermint Core and point it to our application. Staying
within the application directory execute:

```sh
$ TMHOME="/tmp/example" tendermint node --abci grpc --proxy_app tcp://127.0.0.1:26658

I[2019-07-28|15:44:53.632] Version info                                 module=main software=0.32.1 block=10 p2p=7
I[2019-07-28|15:44:53.677] Starting Node                                module=main impl=Node
I[2019-07-28|15:44:53.681] Started node                                 module=main nodeInfo="{ProtocolVersion:{P2P:7 Block:10 App:0} ID_:7639e2841ccd47d5ae0f5aad3011b14049d3f452 ListenAddr:tcp://0.0.0.0:26656 Network:test-chain-Nhl3zk Version:0.32.1 Channels:4020212223303800 Moniker:Ivans-MacBook-Pro.local Other:{TxIndex:on RPCAddress:tcp://127.0.0.1:26657}}"
I[2019-07-28|15:44:54.801] Executed block                               module=state height=8 validTxs=0 invalidTxs=0
I[2019-07-28|15:44:54.814] Committed state                              module=state height=8 txs=0 appHash=0000000000000000
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

```sh
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

`dGVuZGVybWludA==` and `cm9ja3M=` are the base64-encoding of the ASCII of `tendermint` and `rocks` accordingly.

## Outro

I hope everything went smoothly and your first, but hopefully not the last,
Tendermint Core application is up and running. If not, please [open an issue on
Github](https://github.com/tendermint/tendermint/issues/new/choose). To dig
deeper, read [the docs](https://docs.tendermint.com/master/).

The full source code of this example project can be found [here](https://github.com/climber73/tendermint-abci-grpc-kotlin).
