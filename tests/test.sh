
ROOT=$GOPATH/src/github.com/tendermint/tmsp
cd $ROOT

# test golang counter
COUNTER_APP="counter" go run $ROOT/tests/test_counter.go

# test golang counter via grpc
COUNTER_APP="counter -tmsp=grpc" go run $ROOT/tests/test_counter.go -tmsp=grpc

# test nodejs counter
COUNTER_APP="node ../js-tmsp/example/app.js" go run $ROOT/tests/test_counter.go
