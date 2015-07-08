#!/bin/sh

debora open "[::]:46661"
debora --group default.upgrade list # TODO replace with command to test with
echo "Will shut down barak default port..."
sleep 10
debora --group default.upgrade close "[::]:46660"
debora --group default.upgrade run -- bash -c "cd \$GOPATH/src/github.com/tendermint/tendermint; git pull origin develop; make"
debora --group default.upgrade run -- bash -c "cd \$GOPATH/src/github.com/tendermint/tendermint; mkdir -p ~/.barak/logs"
debora --group default.upgrade run --bg --label barak -- bash -c "cd \$GOPATH/src/github.com/tendermint/tendermint; barak --config=cmd/barak/seed | stdinwriter -outpath ~/.barak/logs/barak.log"
echo "Testing new barak..."
debora list
sleep 10
echo "Will shut down old barak..."
debora --group default.upgrade quit
