package rpc

const Maj = "0"
const Min = "5" // refactored out of tendermint/tendermint; RPCResponse.Result is RawJSON
const Fix = "1" // support tcp:// or unix:// prefixes

const Version = Maj + "." + Min + "." + Fix
