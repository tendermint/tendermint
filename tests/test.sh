
ROOT=$GOPATH/src/github.com/tendermint/tmsp
cd $ROOT

# test golang counter
COUNTER_APP="counter" go run $ROOT/tests/test_counter.go

# test nodejs counter
COUNTER_APP="node ../js-tmsp/example/app.js" go run $ROOT/tests/test_counter.go
