# crypto

crypto is the cryptographic package adapted for Tendermint's uses

## Importing it
To get the interfaces,
`import "github.com/tendermint/tendermint/crypto"`

For any specific algorithm, use its specific module e.g.
`import "github.com/tendermint/tendermint/crypto/ed25519"`

If you want to decode bytes into one of the types, but don't care about the specific algorithm, use
`import "github.com/tendermint/tendermint/crypto/amino"`

## Binary encoding

For Binary encoding, please refer to the [Tendermint encoding spec](https://github.com/tendermint/tendermint/blob/master/docs/spec/blockchain/encoding.md).

## JSON Encoding

crypto `.Bytes()` uses Amino:binary encoding, but Amino:JSON is also supported.

```go
Example Amino:JSON encodings:

ed25519.PrivKeyEd25519     - {"type":"954568A3288910","value":"EVkqJO/jIXp3rkASXfh9YnyToYXRXhBr6g9cQVxPFnQBP/5povV4HTjvsy530kybxKHwEi85iU8YL0qQhSYVoQ=="}
ed25519.PubKeyEd25519      - {"type":"AC26791624DE60","value":"AT/+aaL1eB0477Mud9JMm8Sh8BIvOYlPGC9KkIUmFaE="}
crypto.PrivKeySecp256k1   - {"type":"019E82E1B0F798","value":"zx4Pnh67N+g2V+5vZbQzEyRerX9c4ccNZOVzM9RvJ0Y="}
crypto.PubKeySecp256k1    - {"type":"F8CCEAEB5AE980","value":"A8lPKJXcNl5VHt1FK8a244K9EJuS4WX1hFBnwisi0IJx"}
```
