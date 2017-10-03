# Testing and Fuzzing the Peer Layer

### We are using [go-fuzz](https://github.com/dvyukov/go-fuzz) to perform brute force testing

[![](https://img.shields.io/badge/go-1.8-blue.svg)](https://github.com/moovweb/gvm) [![](https://img.shields.io/badge/dependencies-go--fuzz-green.svg)](https://github.com/dvyukov/go-fuzz) [![License](https://img.shields.io/hexpm/l/plug.svg)](https://www.apache.org/licenses/LICENSE-2.0)


### Join the chat!

[![](https://img.shields.io/badge/Rocket.Chat-JOIN%20CHAT-green.svg)](https://cosmos.rocket.chat/)

We have a friendly community of like-minded people that are always eager to help someone in need of advice or just
looking for casual banter.


## Install

1. Download the [Tendermint](https://github.com/tendermint/tendermint) source code:
```
$ go get -u github.com/tendermint/tendermint
```


2. Download the two [go-fuzz](https://github.com/dvyukov/go-fuzz) tools.

The go-fuzz fuzzer:
```
$ go get -u github.com/dvyukov/go-fuzz/go-fuzz
```


And the go-fuzz-build program that will produce the test archive:
```
$ go get -u github.com/dvyukov/go-fuzz/go-fuzz-build
```


At this point, the two go-fuzz tools will be in your *$GOPATH/bin* directory.


## Running the test

1. Create a test directory where go-fuzz will store files and make it your current working directory:
```
$ mkdir $GOPATH/testdir
$ cd $GOPATH/testdir
```


2. Build the go-fuzz test archive:
```
$ $GOPATH/bin/go-fuzz-build github.com/tendermint/tendermint/p2p
```


If step 6 still doesn't show the fuzzer firing successful executions, you may need to build the archive like this:
```
$ CGO_ENABLED=0 $GOPATH/bin/go-fuzz-build github.com/tendermint/tendermint/p2p
```


3. Jail the fuzzer into a network namespace that cannot reach the Internet:
```
$ sudo unshare -n -- sh -c 'ifconfig lo up; bash'
$ su *username*
$ source ~/.profile
```

We do not want the PEXReactor attempting to reach our randomly generated peers.


4. Execute the fuzzer:
```
$ $GOPATH/bin/go-fuzz -bin=./p2p-fuzz.zip -workdir=./
```

It is normal for this test to use a large amount of your machine's available CPU cycles.


## The output produced

The fuzzer will continuously output results similar to the following:
```
2017/09/20 20:31:25 slaves: 8, corpus: 94 (17m32s ago), crashers: 0, 
    restarts: 1/9967, execs: 38843712 (21296/sec), cover: 962, uptime: 30m24s
```

The corpus and cover will slowly increase as the test continues to run, which indicates that additional discoveries are being made by the fuzzer.


## Cleaning up the test directory

Keeping the **corpus**, **crashers**, and **suppressions** directories in between runs of the fuzzer allows it to pick up where it left off.

To start "fresh," remove those directories from the test directory.