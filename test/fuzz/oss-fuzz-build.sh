#!/bin/bash
# This script is invoked by OSS-Fuzz to run fuzz tests against Tendermint core.
# See https://github.com/google/oss-fuzz/blob/master/projects/tendermint/build.sh

compile_go_fuzzer github.com/tendermint/tendermint/test/fuzz/mempool/v0 Fuzz mempool_v0_fuzzer
compile_go_fuzzer github.com/tendermint/tendermint/test/fuzz/mempool/v1 Fuzz mempool_v1_fuzzer
compile_go_fuzzer github.com/tendermint/tendermint/test/fuzz/p2p/addrbook Fuzz p2p_addrbook_fuzzer
compile_go_fuzzer github.com/tendermint/tendermint/test/fuzz/p2p/pex Fuzz p2p_pex_fuzzer
compile_go_fuzzer github.com/tendermint/tendermint/test/fuzz/p2p/secret_connection Fuzz p2p_secret_connection_fuzzer
compile_go_fuzzer github.com/tendermint/tendermint/test/fuzz/rpc/jsonrpc/server Fuzz rpc_jsonrpc_server_fuzzer
