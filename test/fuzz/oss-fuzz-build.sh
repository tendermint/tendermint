#!/bin/bash
# This script is invoked by OSS-Fuzz to run fuzz tests against Tendermint core.
# See https://github.com/google/oss-fuzz/blob/master/projects/tendermint/build.sh
set -euo pipefail

# Upgrade to Go 1.18. Remove when it's the default.
apt-get update && apt-get install -y wget
wget https://go.dev/dl/go1.18.2.linux-amd64.tar.gz

mkdir -p temp-go
rm -rf /root/.go/*
tar -C temp-go/ -xzf go1.18.2.linux-amd64.tar.gz
mv temp-go/go/* /root/.go/

export FUZZ_ROOT="github.com/tendermint/tendermint"

build_go_fuzzer() {
	local function="$1"
	local fuzzer="$2"

	go run github.com/orijtech/otils/corpus2ossfuzz@latest -o "$OUT"/"$fuzzer"_seed_corpus.zip -corpus test/fuzz/tests/testdata/fuzz/"$function"
	compile_native_go_fuzzer "$FUZZ_ROOT"/test/fuzz/tests "$function" "$fuzzer"
}

go get github.com/AdamKorcz/go-118-fuzz-build/utils
go get github.com/prometheus/common/expfmt@v0.32.1

build_go_fuzzer FuzzP2PSecretConnection fuzz_p2p_secretconnection

build_go_fuzzer FuzzMempool fuzz_mempool

build_go_fuzzer FuzzRPCJSONRPCServer fuzz_rpc_jsonrpc_server
