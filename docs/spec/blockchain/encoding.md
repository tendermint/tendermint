# Tendermint Encoding

## Amino

Tendermint uses the proto3 derivative [Amino](https://github.com/tendermint/go-amino) for all data structures.
Think of Amino as an object-oriented proto3 with native JSON support.
The goal of the Amino encoding protocol is to bring parity between application
logic objects and persistence objects.

Please see the [Amino
specification](https://github.com/tendermint/go-amino#amino-encoding-for-go) for
more details.

Notably, every object that satisfies an interface (eg. a particular kind of p2p message,
or a particular kind of pubkey) is registered with a global name, the hash of
which is included in the object's encoding as the so-called "prefix bytes".

We define the `func AminoEncode(obj interface{}) []byte` function to take an
arbitrary object and return the Amino encoded bytes.

## Byte Arrays

The encoding of a byte array is simply the raw-bytes prefixed with the length of
the array as a `UVarint` (what proto calls a `Varint`).

For details on varints, see the [protobuf
spec](https://developers.google.com/protocol-buffers/docs/encoding#varints).

For example, the byte-array `[0xA, 0xB]` would be encoded as `0x020A0B`,
while a byte-array containing 300 entires beginning with `[0xA, 0xB, ...]` would
be encoded as `0xAC020A0B...` where `0xAC02` is the UVarint encoding of 300.

## Public Key Cryptography

Tendermint uses Amino to distinguish between different types of private keys,
public keys, and signatures. Additionally, for each public key, Tendermint
defines an Address function that can be used as a more compact identifier in
place of the public key. Here we list the concrete types, their names,
and prefix bytes for public keys and signatures, as well as the address schemes
for each PubKey. Note for brevity we don't
include details of the private keys beyond their type and name, as they can be
derived the same way as the others using Amino.

All registered objects are encoded by Amino using a 4-byte PrefixBytes that
uniquely identifies the object and includes information about its underlying
type. For details on how PrefixBytes are computed, see the [Amino
spec](https://github.com/tendermint/go-amino#computing-the-prefix-and-disambiguation-bytes).

In what follows, we provide the type names and prefix bytes directly.
Notice that when encoding byte-arrays, the length of the byte-array is appended
to the PrefixBytes. Thus the encoding of a byte array becomes `<PrefixBytes>
<Length> <ByteArray>`. In other words, to encode any type listed below you do not need to be
familiar with amino encoding.
You can simply use below table and concatenate Prefix || Length (of raw bytes) || raw bytes
( while || stands for byte concatenation here).

| Type | Name | Prefix | Length | Notes |
| ---- | ---- | ------ | ----- | ------ |
| PubKeyEd25519 | tendermint/PubKeyEd25519 | 0x1624DE64 | 0x20 |  |
| PubKeySecp256k1 | tendermint/PubKeySecp256k1 | 0xEB5AE987 | 0x21 |  |
| PrivKeyEd25519 | tendermint/PrivKeyEd25519 | 0xA3288910 | 0x40 |  |
| PrivKeySecp256k1 | tendermint/PrivKeySecp256k1 | 0xE1B0F79B | 0x20 |  |
| SignatureEd25519 | tendermint/SignatureEd25519 | 0x2031EA53 | 0x40 |  |
| SignatureSecp256k1 | tendermint/SignatureSecp256k1 | 0x7FC4A495 | variable |
|

### Examples

1. For example, the 33-byte (or 0x21-byte in hex) Secp256k1 pubkey
`020BD40F225A57ED383B440CF073BC5539D0341F5767D2BF2D78406D00475A2EE9`
would be encoded as
`EB5AE98221020BD40F225A57ED383B440CF073BC5539D0341F5767D2BF2D78406D00475A2EE9`

2. For example, the variable size Secp256k1 signature (in this particular example 70 or 0x46 bytes)
`304402201CD4B8C764D2FD8AF23ECFE6666CA8A53886D47754D951295D2D311E1FEA33BF02201E0F906BB1CF2C30EAACFFB032A7129358AFF96B9F79B06ACFFB18AC90C2ADD7`
would be encoded as
`16E1FEEA46304402201CD4B8C764D2FD8AF23ECFE6666CA8A53886D47754D951295D2D311E1FEA33BF02201E0F906BB1CF2C30EAACFFB032A7129358AFF96B9F79B06ACFFB18AC90C2ADD7`

### Addresses

Addresses for each public key types are computed as follows:

#### Ed25519

First 20-bytes of the SHA256 hash of the raw 32-byte public key:

```
address = SHA256(pubkey)[:20]
```

NOTE: before v0.22.0, this was the RIPEMD160 of the Amino encoded public key.

#### Secp256k1

RIPEMD160 hash of the SHA256 hash of the OpenSSL compressed public key:

```
address = RIPEMD160(SHA256(pubkey))
```

This is the same as Bitcoin.

## Other Common Types

### BitArray

The BitArray is used in block headers and some consensus messages to signal
whether or not something was done by each validator. BitArray is represented
with a struct containing the number of bits (`Bits`) and the bit-array itself
encoded in base64 (`Elems`).

```go
type BitArray struct {
    Bits  int
    Elems []uint64
}
```

This type is easily encoded directly by Amino.

Note BitArray receives a special JSON encoding in the form of `x` and `_`
representing `1` and `0`. Ie. the BitArray `10110` would be JSON encoded as
`"x_xx_"`

### Part

Part is used to break up blocks into pieces that can be gossiped in parallel
and securely verified using a Merkle tree of the parts.

Part contains the index of the part in the larger set (`Index`), the actual
underlying data of the part (`Bytes`), and a simple Merkle proof that the part is contained in
the larger set (`Proof`).

```go
type Part struct {
    Index int
    Bytes byte[]
    Proof byte[]
}
```

### MakeParts

Encode an object using Amino and slice it into parts.

```go
func MakeParts(obj interface{}, partSize int) []Part
```

## Merkle Trees

Simple Merkle trees are used in numerous places in Tendermint to compute a cryptographic digest of a data structure.

Tendermint always uses the `TMHASH` hash function, which is the first 20-bytes
of the SHA256:

```
func TMHASH(bz []byte) []byte {
    shasum := SHA256(bz)
    return shasum[:20]
}
```

### Simple Merkle Root

The function `SimpleMerkleRoot` is a simple recursive function defined as follows:

```go
func SimpleMerkleRoot(hashes [][]byte) []byte{
    switch len(hashes) {
    case 0:
        return nil
    case 1:
        return hashes[0]
    default:
        left := SimpleMerkleRoot(hashes[:(len(hashes)+1)/2])
        right := SimpleMerkleRoot(hashes[(len(hashes)+1)/2:])
        return SimpleConcatHash(left, right)
    }
}

func SimpleConcatHash(left, right []byte) []byte{
    left = encodeByteSlice(left)
    right = encodeByteSlice(right)
    return TMHASH(append(left, right))
}
```

Note that the leaves are Amino encoded as byte-arrays (ie. simple Uvarint length
prefix) before being concatenated together and hashed.

Note: we will abuse notion and invoke `SimpleMerkleRoot` with arguments of type `struct` or type `[]struct`.
For `struct` arguments, we compute a `[][]byte` containing the hash of each
field in the struct sorted by the hash of the field name.
For `[]struct` arguments, we compute a `[][]byte` by hashing the individual `struct` elements.

### Simple Merkle Proof

Proof that a leaf is in a Merkle tree consists of a simple structure:


```
type SimpleProof struct {
        Aunts [][]byte
}
```

Which is verified using the following:

```
func (proof SimpleProof) Verify(index, total int, leafHash, rootHash []byte) bool {
	computedHash := computeHashFromAunts(index, total, leafHash, proof.Aunts)
    return computedHash == rootHash
}

func computeHashFromAunts(index, total int, leafHash []byte, innerHashes [][]byte) []byte{
	assert(index < total && index >= 0 && total > 0)

	if total == 1{
		assert(len(proof.Aunts) == 0)
		return leafHash
	}

	assert(len(innerHashes) > 0)

	numLeft := (total + 1) / 2
	if index < numLeft {
		leftHash := computeHashFromAunts(index, numLeft, leafHash, innerHashes[:len(innerHashes)-1])
		assert(leftHash != nil)
		return SimpleHashFromTwoHashes(leftHash, innerHashes[len(innerHashes)-1])
	}
	rightHash := computeHashFromAunts(index-numLeft, total-numLeft, leafHash, innerHashes[:len(innerHashes)-1])
	assert(rightHash != nil)
	return SimpleHashFromTwoHashes(innerHashes[len(innerHashes)-1], rightHash)
}
```

## JSON

### Amino

TODO: improve this

Amino also supports JSON encoding - registered types are simply encoded as:

```
{
  "type": "<DisfixBytes>",
  "value": <JSON>
}
```

For instance, an ED25519 PubKey would look like:

```
{
  "type": "tendermint/PubKeyEd25519",
  "value": "uZ4h63OFWuQ36ZZ4Bd6NF+/w9fWUwrOncrQsackrsTk="
}
```

Where the `"value"` is the base64 encoding of the raw pubkey bytes, and the
`"type"` is the full disfix bytes for Ed25519 pubkeys.


### Signed Messages

Signed messages (eg. votes, proposals) in the consensus are encoded using Amino-JSON, rather than in the standard binary format.

When signing, the elements of a message are sorted by key and the sorted message is embedded in an
outer JSON that includes a `chain_id` field.
We call this encoding the CanonicalSignBytes. For instance, CanonicalSignBytes for a vote would look
like:

```json
{"chain_id":"my-chain-id","vote":{"block_id":{"hash":DEADBEEF,"parts":{"hash":BEEFDEAD,"total":3}},"height":3,"round":2,"timestamp":1234567890, "type":2}
```

Note how the fields within each level are sorted.
