# loadtime

This directory contains `loadtime`, a tool for generating transaction load against Tendermint.
`loadtime` generates transactions that contain the timestamp corresponding to when they were generated
as well as additional metadata to track the variables used when generating the load.


## Building loadtime

The `Makefile` contains a target for building the `loadtime` tool.

The following command will build the tool and place the resulting binary in `./build/loadtime`.

```bash
make build
```

## Use

`loadtime` leverages the [tm-load-test](https://github.com/informalsystems/tm-load-test)
framework. As a result, all flags and options specified on the `tm-load-test` apply to
`loadtime`.

Below is a basic invocation for generating load against a Tendermint websocket running
on `localhost:25567`

```bash
loadtime \
    -c 1 -T 10 -r 1000 -s 1024 \
    --broadcast-tx-method sync \
    --endpoints ws://localhost:26657/websocket
```
