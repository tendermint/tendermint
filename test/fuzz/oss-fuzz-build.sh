#!/bin/bash -eu

export FUZZ_ROOT="github.com/tendermint/tendermint"

(cd test/fuzz/p2p/addrbook; go run ./init-corpus/main.go)
compile_go_fuzzer "$FUZZ_ROOT"/test/fuzz/p2p/addrbook Fuzz fuzz_p2p_addrbook fuzz
(cd test/fuzz/p2p/pex; go run ./init-corpus/main.go)
compile_go_fuzzer "$FUZZ_ROOT"/test/fuzz/p2p/pex Fuzz fuzz_p2p_pex fuzz
(cd test/fuzz/p2p/secret_connection; go run ./init-corpus/main.go)
compile_go_fuzzer "$FUZZ_ROOT"/test/fuzz/p2p/secret_connection Fuzz fuzz_p2p_secret_connection fuzz

compile_go_fuzzer "$FUZZ_ROOT"/test/fuzz/mempool Fuzz fuzz_mempool fuzz

compile_go_fuzzer "$FUZZ_ROOT"/test/fuzz/rpc/jsonrpc/server Fuzz fuzz_rpc_jsonrpc_server fuzz
