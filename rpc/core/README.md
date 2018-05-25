# Tendermint RPC

## Generate markdown for [Slate](https://github.com/tendermint/slate)

We are using [Slate](https://github.com/tendermint/slate) to power our RPC
documentation. For generating markdown use:

```shell
go get github.com/davecheney/godoc2md

godoc2md -template rpc/core/doc_template.txt github.com/tendermint/tendermint/rpc/core | grep -v -e "pipe.go" -e "routes.go" -e "dev.go" | sed 's$/src/target$https://github.com/tendermint/tendermint/tree/master/rpc/core$'
```

For more information see the [CI script for building the Slate docs](/scripts/slate.sh)

## Pagination

Requests that return multiple items will be paginated to 30 items by default.
You can specify further pages with the ?page parameter. You can also set a
custom page size up to 100 with the ?per_page parameter.
