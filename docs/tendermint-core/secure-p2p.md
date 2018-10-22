# Secure P2P

The Tendermint p2p protocol uses an authenticated encryption scheme
based on the [Station-to-Station
Protocol](https://en.wikipedia.org/wiki/Station-to-Station_protocol).

Each peer generates an ED25519 key-pair to use as a persistent
(long-term) id.

When two peers establish a TCP connection, they first each generate an
ephemeral X25519 key-pair to use for this session, and send each other
their respective ephemeral public keys. This happens in the clear.

They then each compute the shared secret, as done in a [diffie hellman
key exhange](https://en.wikipedia.org/wiki/Diffie%E2%80%93Hellman_key_exchange).
The shared secret is used as the symmetric key for the encryption algorithm.

We then run [hkdf-sha256](https://en.wikipedia.org/wiki/HKDF) to expand the
shared secret to generate a symmetric key for sending data,
a symmetric key for receiving data,
a challenge to authenticate the other party.
One peer will send data with their sending key, and the other peer
would decode it using their own receiving key.
We must ensure that both parties don't try to use the same key as the sending
key, and the same key as the receiving key, as in that case nothing can be
decoded.
To ensure this, the peer with the canonically smaller ephemeral pubkey
uses the first key as their receiving key, and the second key as their sending key.
If the peer has the canonically larger ephemeral pubkey, they do the reverse.

Each peer also keeps a received message counter and sent message counter, both
are initialized to zero.
All future communication is encrypted using chacha20poly1305.
The key used to send the message is the sending key, and the key used to decode
the message is the receiving key.
The nonce for chacha20poly1305 is the relevant message counter.
It is critical that the message counter is incremented every time you send a
message and every time you receive a message that decodes correctly.

Each peer now signs the challenge with their persistent private key, and
sends the other peer an AuthSigMsg, containing their persistent public
key and the signature. On receiving an AuthSigMsg, the peer verifies the
signature.

The peers are now authenticated.

The communication maintains Perfect Forward Secrecy, as
the persistent key pair was not used for generating secrets - only for
authenticating.

## Caveat

This system is still vulnerable to a Man-In-The-Middle attack if the
persistent public key of the remote node is not known in advance. The
only way to mitigate this is with a public key authentication system,
such as the Web-of-Trust or Certificate Authorities. In our case, we can
use the blockchain itself as a certificate authority to ensure that we
are connected to at least one validator.

## Config

Authenticated encryption is enabled by default.

## Specification

The full p2p specification can be found [here](https://github.com/tendermint/tendermint/tree/master/docs/spec/p2p).

## Additional Reading

- [Implementation](https://github.com/tendermint/tendermint/blob/64bae01d007b5bee0d0827ab53259ffd5910b4e6/p2p/conn/secret_connection.go#L47)
- [Original STS paper by Whitfield Diffie, Paul C. van Oorschot and
  Michael J.
  Wiener](http://citeseerx.ist.psu.edu/viewdoc/download?doi=10.1.1.216.6107&rep=rep1&type=pdf)
- [Further work on secret
  handshakes](https://dominictarr.github.io/secret-handshake-paper/shs.pdf)
