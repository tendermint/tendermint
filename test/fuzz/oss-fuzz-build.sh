#!/bin/bash -eu

export FUZZ_ROOT="github.com/tendermint/tendermint"

(cd test/fuzz/p2p/secretconnection; go run ./init-corpus/main.go)
compile_go_fuzzer "$FUZZ_ROOT"/test/fuzz/p2p/secretconnection Fuzz fuzz_p2p_secretconnection fuzz

compile_go_fuzzer "$FUZZ_ROOT"/test/fuzz/mempool Fuzz fuzz_mempool fuzz

compile_go_fuzzer "$FUZZ_ROOT"/test/fuzz/rpc/jsonrpc/server Fuzz fuzz_rpc_jsonrpc_server fuzz
