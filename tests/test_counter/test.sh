# These tests spawn the counter app and server by execing the COUNTER_APP command and run some simple client tests against it

ROOT=$GOPATH/src/github.com/tendermint/tmsp/tests/test_counter
cd $ROOT

# test golang counter
COUNTER_APP="counter" go run test_counter.go

# test golang counter via grpc
COUNTER_APP="counter -tmsp=grpc" go run test_counter.go -tmsp=grpc

# test nodejs counter
COUNTER_APP="node $GOPATH/src/github.com/tendermint/js-tmsp/example/app.js" go run test_counter.go
