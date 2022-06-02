# ABCI Dump

## Overview

ABCI Dump dumps and parses Protobuf communication. It is designed to dump traffic TCP
between ABCI App and Tendermint/Tenderdash, parse and display it. It also supports
parsing hex-encoded and base64-encoded Protobuf messages.

ABCI Dump can:

* dump live traffic (just as `tcpdump`), parsing and displaying it as JSON,
* parse data passed in hex or base64 format (both length-delimited and raw protobuf data),
* parse and display base64-encoded data in CBOR format.

## Build and install

### Installing dependencies

ABCI Dump requires `libpcap` to operate, and `libpcap-dev` to build:

```bash
apt-get install -y libpcap0.8 libpcap-dev
```


### Building from source

Just call:

```
make build_abcidump
```

or:

```
make install_abcidump
```

### Using with Docker

Since Tenderdash 0.7.2, ABCI Dump is installed in the docker image as /usr/bin/abcidump

## Usage

### Capturing live TCP traffic

1. Ensure ABCI communication uses TCP and find the TCP port in use. To do this, check the  `proxy_app` 
setting in `config.toml`:

   ```toml
   proxy_app = "tcp://drive_abci:26658"
    ```

2. Start ABCI Capture on the interface between Tenderdash and ABCI App (here `eth0`):

    ```bash
    sudo ./abcidump capture -i eth0 -p 26658
    ```

3. Observe the results. You can also save results to a file, redirecting stdout:

    ```bash
    sudo ./abcidump capture -i eth0 -p 26658 | tee log.json
    ```

### Parsing protobuf messages

ABCI Dump supports parsing dumped protobuf messages, both raw as well as length-delimited.

For example:
 
```bash
sudo abcidump \
    --type "tendermint.abci.Request" \
        --format hex \
        --input "c60232a0010a8a0108d0a4a1a1011220c08d20981256fe2691095159612d9e55dc28aeb086cce8bd0000171f400164421a60fdb7e62f6400000085371e73acc009e86e4e6463714e1273e3a735fb4f544f1caa6f7a20b2c5608a32041e1d7e787ffa400cbfca62f49e3813b5460881464851890b1bf4a0ab170158099243f1250933f316c287f2e1db67134e5acc96aeca1812112f7665726966792d636861696e6c6f636b041200"
```

The input payload above contains the following query request:

```json
{
	"query": {
		"data": "CNCkoaEBEiDAjSCYElb+JpEJUVlhLZ5V3CiusIbM6L0AABcfQAFkQhpg/bfmL2QAAACFNx5zrMAJ6G5OZGNxThJz46c1+09UTxyqb3ogssVgijIEHh1+eH/6QAy/ymL0njgTtUYIgUZIUYkLG/SgqxcBWAmSQ/ElCTPzFsKH8uHbZxNOWsyWrsoY",
		"path": "/verify-chainlock"
	}
}
```

where "data" is a base64 of protobuf-encoded `tendermint.types.CoreChainLock`, and can be decoded in `raw` mode (that is, without length prefix), as follows:

```bash
abcidump parse --format base64 --input "CNCkoaEBEiDAjSCYElb+JpEJUVlhLZ5V3CiusIbM6L0AABcfQAFkQhpg/bfmL2QAAACFNx5zrMAJ6G5OZGNxThJz46c1+09UTxyqb3ogssVgijIEHh1+eH/6QAy/ymL0njgTtUYIgUZIUYkLG/SgqxcBWAmSQ/ElCTPzFsKH8uHbZxNOWsyWrsoY" --raw  --type tendermint.types.CoreChainLock
{
        "coreBlockHeight": 338186832,
        "coreBlockHash": "wI0gmBJW/iaRCVFZYS2eVdworrCGzOi9AAAXH0ABZEI=",
        "signature": "/bfmL2QAAACFNx5zrMAJ6G5OZGNxThJz46c1+09UTxyqb3ogssVgijIEHh1+eH/6QAy/ymL0njgTtUYIgUZIUYkLG/SgqxcBWAmSQ/ElCTPzFsKH8uHbZxNOWsyWrsoY"
}
```

### Decoding CBOR messages

Some responses are encoded in CBOR notation. See https://cbor.me/ for more details.

To decode CBOR, use the following command:

```bash
abcidump cbor decode \
    --format base64 \
    --input 'omdtZXNzYWdleB1DaGFpbkxvY2sgdmVyaWZpY2F0aW9uIGZhaWxlZGRkYXRho2ZoZWlnaHQaJGELXWlibG9ja0hhc2h4QDI1MTc3MzgwZDcyMjdjODcyNWFmNDJlMTlkMWFjNDdhYWViMjZkOTM2YjQwMzQ1MDAwMDAxNTI3ZTBmNjQ5NzVpc2lnbmF0dXJleMA5YTZmNmUxMWUyMDAwMDAwMTNkZmFiNDZjMmEzMWE2ZGRlZGRiYmNjNzQ3OTMzMzBlODI1MTliMTZiNGQyMjYwYWViMDU5MWViMjI3NDAxZjdjMTIxOTU2NWYwMzFlZDg0MzQ0NjVjNjkxZjM4Y2E5MTZhZmI5ZDlmYzViZjIwZGM4ODMxMjdhYmI3MWRmNTE1MzI3ZWQzMGIxZTI2Y2I0ZTNlNjRmN2FmNWY0ODJhODBhYzAyODM4NjRkNjY2ZDI='
```

### Using inside Docker container

To use, ABCI Dump inside a Docker container, execute the following command:

```bash
docker exec --user root -ti [image_id] /usr/bin/abcidump
```

For example:

```bash
docker exec --user root -ti mn_evo_services_tendermint_1 /usr/bin/abcidump
```
