#! /bin/bash

cd $GOPATH/src/github.com/tendermint/go-rpc

# get deps
go get -u -t ./...

# go tests
go test --race github.com/tendermint/go-rpc/...


# integration tests
cd test
set -e

go build -o server main.go
./server > /dev/null &
PID=$!
sleep 2

# simple request
R1=`curl -s 'http://localhost:8008/hello_world?name="my_world"&num=5'`
R2=`curl -s --data @data.json http://localhost:8008`
if [[ "$R1" != "$R2" ]]; then
	echo "responses are not identical:"
	echo "R1: $R1"
	echo "R2: $R2"
	exit 1
else
	echo "Success"
fi

# request with 0x-prefixed hex string arg
R1=`curl -s 'http://localhost:8008/hello_world?name=0x41424344&num=123'`
R2='{"jsonrpc":"2.0","id":"","result":{"Result":"hi ABCD 123"},"error":""}'
if [[ "$R1" != "$R2" ]]; then
	echo "responses are not identical:"
	echo "R1: $R1"
	echo "R2: $R2"
	exit 1
else
	echo "Success"
fi

# request with unquoted string arg
R1=`curl -s 'http://localhost:8008/hello_world?name=abcd&num=123'`
R2="{\"jsonrpc\":\"2.0\",\"id\":\"\",\"result\":null,\"error\":\"Error converting http params to args: invalid character 'a' looking for beginning of value\"}"
if [[ "$R1" != "$R2" ]]; then
	echo "responses are not identical:"
	echo "R1: $R1"
	echo "R2: $R2"
	exit 1
else
	echo "Success"
fi

# request with string type when expecting number arg
R1=`curl -s 'http://localhost:8008/hello_world?name="abcd"&num=0xabcd'`
R2="{\"jsonrpc\":\"2.0\",\"id\":\"\",\"result\":null,\"error\":\"Error converting http params to args: Got a hex string arg, but expected 'int'\"}"
if [[ "$R1" != "$R2" ]]; then
	echo "responses are not identical:"
	echo "R1: $R1"
	echo "R2: $R2"
	exit 1
else
	echo "Success"
fi

kill -9 $PID || exit 0
