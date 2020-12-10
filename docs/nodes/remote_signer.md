---
order: 7
---

# Remote signer

Tendermint provides a remote signer option for validators. A remote signer enables the operator to store the validator key on a different machine minimizing the attack surface if a server were to be compromised.

The remote signer protocol implements a [client and server architecture](https://en.wikipedia.org/wiki/Client%E2%80%93server_model). The node is the client and the remote signer is the server. When Tendermint requires the public key or signature for a proposal or vote it requests it from the server.

There are two different configurations that can be used: raw or gRPC.

## Raw

While both options use tcp or unix sockets the raw option uses tcp or unix sockets without http. Raw is the old protocol for remote signers.

> Warning: Raw will be deprecated in a future major release, we recommend implementing your key management server against the gRPC configuration.

## gRPC

[gRPC](https://grpc.io/) is an RPC framework built with [HTTP/2](https://en.wikipedia.org/wiki/HTTP/2), uses [Protocol Buffers](https://developers.google.com/protocol-buffers) to define services and has been standardized within the cloud infrastructure community. gRPC provides a language agnostic way to implement services. This aids developers in the writing key management servers in various different languages.

GRPC utilizes [TLS](https://en.wikipedia.org/wiki/Transport_Layer_Security), another widely standardized protocol, to secure connections. There are two forms of TLS to secure a connection, one-way and two-way. One way is when the client identifies the server but the server allows anyone to connect to it. Two-way is when the client identifies the server and the server identifies the client, prohibiting connections from unknown parties.

Securing your remote signers connection is highly recommended, but we provide the option to run it with a insecure connection.

### Generating Certificates

To run a secure connection with gRPC we need to generate certificates and keys. We will walkthrough how to self sign certificates for two-way TLS.

There are two ways to generate certificates, [openssl](https://www.openssl.org/) and [certstarp](https://github.com/square/certstrap). Both of these options can be used but we will be covering `certstrap` because it provides a simpler process then openssl.

- Install `Certstrap`:

```sh
  go get github.com/square/certstrap@v1.2.0
```

- Create certificate authority for self signing.

```sh
 # generate self signing ceritificate authority
 certstrap init --common-name "<name_CA>" --expires "20 years"
```

- Request a certificate for the server.
  - For generalization purposes we set the ip to `127.0.0.1`, but for your node please use the servers IP.
- Sign the servers certificate with your certificate authority

```sh
 # generate server cerificate
 certstrap request-cert -cn server -ip 127.0.0.1
 # self-sign server cerificate with rootCA
 certstrap sign server --CA "<name_CA>" 127.0.0.1
  ```

- Request a certificate for the client.
  - For generalization purposes we set the ip to `127.0.0.1`, but for your node please use the clients IP.
- Sign the clients certificate with your certificate authority

```sh
# generate client cerificate
 certstrap request-cert -cn client -ip 127.0.0.1
# self-sign client cerificate with rootCA
 certstrap sign client --CA "<name_CA>" 127.0.0.1
```
