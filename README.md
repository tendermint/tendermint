[**Website**](http://tendermint.com) **|** 
[**Blog**](http://tendermint.com/posts/) **|**
[**Forum**] (http://forum.tendermint.com) **|**
[**IRC**] (http://webchat.freenode.net/?randomnick=1&channels=%23tendermint)

Tendermint in Golang

Tendermint is a completely decentralized byzantine consensus protocol suitable for use in cryptocurrencies.

This project is a reference implementation of the protocol.

## Submodules

* **[consensus](https://github.com/tendermint/tendermint/blob/master/consensus):** core consensus algorithm
* **[state](https://github.com/tendermint/tendermint/blob/master/state):** application state; mutated by transactions
* **[blocks](https://github.com/tendermint/tendermint/blob/master/blocks):** structures of the blockchain
* **[mempool](https://github.com/tendermint/tendermint/blob/master/mempool):** gossip of new transactions
* **[merkle](https://github.com/tendermint/tendermint/blob/master/merkle):** merkle hash trees
* **[p2p](https://github.com/tendermint/tendermint/blob/master/p2p):**  extensible P2P networking

## Requirements

[Go](http://golang.org) 1.2 or newer.

OpenSSL header files. `sudo apt-get install libssl-dev` on Debian/Ubuntu
//TODO OpenSSL header files for other platforms


###Setting up Golang

[Install Go for your platform](https://golang.org/doc/install)

Set up environment variables. Go requires certain environment variables to compile code. Set these in your terminal, .profile or .zshrc as appropiate.

```
export GOROOT=$HOME/go
export PATH=$PATH:$GOROOT/bin
export GOPATH=$HOME/gopkg
```

## Build

[![Build Status](https://drone.io/github.com/tendermint/tendermint/status.png)](https://drone.io/github.com/tendermint/tendermint/latest)

```
make get_deps
make
```

## Run

`./tendermint daemon --help`



### Editing your config.json

The file will be create in ~/.tendermint/config.json

There is not official or testnet SeedNode yet. Will updated with an official list of seed nodes.

//TODO Explanation of other config.json fields

```
{
        "Network": "tendermint_testnet0",
        "LAddr": "0.0.0.0:0",
        "SeedNode": "",
        "DB": {
                "Backend": "leveldb",
                "Dir": "/home/zaki/.tendermint/data"
        },
        "Alert": {
                "MinInterval": 0,
                "TwilioSid": "",
                "TwilioToken": "",
                "TwilioFrom": "",
                "TwilioTo": "",
                "EmailRecipients": null
        },
        "SMTP": {
                "User": "",
                "Password": "",
                "Host": "",
                "Port": 0
        },
        "RPC": {
                "HTTPLAddr": "0.0.0.0:0"
        }
}

```

You will also to need to have a genesis.json in ~/.tendermint/. This must be the common genesis.json as the network you are joining from the Seed Node

```
{
    "Accounts": [
        {
            "Address": "6070ff17c39b2b0a64ca2bc431328037fa0f4760",
            "Amount":  200000000
        }
    ],
    "Validators": [
        {
            "PubKey": "01206bd490c212e701a2136eeea04f06fa4f287ee47e2b7a9b5d62edd84cd6ad9753",
            "Amount": 100000000,
            "UnbondTo": [
                {
                        "Address": "6070ff17c39b2b0a64ca2bc431328037fa0f4760",
                        "Amount":  100000000
                }
            ]
        }
    ]
}
```

## Resources

IRC Channel: #tendermint on freenode
