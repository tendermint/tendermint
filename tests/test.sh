
ROOT=$GOPATH/src/github.com/tendermint/tmsp
cd $ROOT

# test golang dummy
bash tests/test_dummy.sh

# test golang counter
bash tests/test_counter.sh

# test python counter
cd example/python
COUNTER_APP="python app.py" bash $ROOT/tests/test_counter.sh
