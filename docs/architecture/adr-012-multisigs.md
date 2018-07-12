# ADR 012: Multisignatures

## Context

Multisignatures, or technically _Accountable Subgroup Multisignatures_ (ASM), are signature schemes which enable any subgroup of a set of signers to sign any message, and reveal to the verifier exactly who the signers were. 
This allows for complex conditionals of when to validate a signature.

Suppose the set of signers is of size _n_. 
If we validate a signature if any subgroup of size _k_ signs a message, this becomes what is commonly reffered to as a _k of n multisig_ in Bitcoin. 
We can also allow for more complex conditionals, such as weighting signers, and requiring a certain weighted threshold to be reached. 

Accountable Subgroup Multisignatures are a feature we would like to support in go-crypto, and more specifically k of n threshold signatures. 
This ADR specifies the encoding standard for general accountable subgroup multisignatures, k of n multisigs, and its weighted variant. 

## Proposed Solution

### New structs

This ADR assumes we have a var uint encoding of verification types for pubkeys, as proposed [here](https://github.com/tendermint/tendermint/issues/1957). 
We should have all ASM enum types have the same first byte prefix (perhaps `\x80`, as it must be `>= \x80` for var uint encoding to expect a second byte).

This verification type idea is similar to the idea used in [Kadena](https://github.com/kadena-io/pact/blob/master/docs/pact-reference.rst#keysets-and-authorization), as in our case we will have different verification types for the different ways we validate the accountable subset. We are encoding differently however. 
The reason for having a shared first byte here rather than sharing the main pubkey enum type proposed here is to prevent a large proliferation of similarly named pubkey types under the main enum, and to have some grouping of signatures of similar type.

In golang, this would look like: 
```
type VerificationType uint
func MarshalVerificationType(VerificationType) []byte
func UnmarshalVerificationType([]byte]) (VerificationType, error)

func VerificationTypeToString(VerificationType) string

const (
	KofNMultisig             VerificationType = 128 // encodes to 0x8000
	WeightedKofNMultisig     VerificationType = 129 // encodes to 0x8001
)
``` 
(The verification type is proposed seperately in the referrenced github issue)

We should provide the following methods in addition to the enum. 
These will be specified in the encoding section, and are intended for usage in most ASM's. 
(In the following I assume that crypto.Signature is of type []byte, it can be switched with crypto.Signature) 
```
func MarshalKeys([]crypto.Pubkey) []byte
func UnmarshalKeys([]byte) ([]crypto.Pubkey, error)
func MarshalBitArray(indices []int, n int) []byte
func UnmarshalBitArray([]byte, n int) (indices []int)
func MarshalSignatures([][]byte) []byte
func UnmarshalSignatures([]byte) ([][]byte)
```

Every ASM will then have its own struct, implementing the crypto.pubkey interface. 
It is recommended that (if sensible), pubkey and signature encodings use the provided marshal keys, and marshal signatures methods to marshal that sub-component of the signature. 
Not requiring these methods be used of all ASM's allows support of fancy future ASM variants. 
Boneh's [BLS ASM](https://eprint.iacr.org/2018/483.pdf) is a notable example, as it only has a single aggregated pubkey, and consequently will not be required to use this marshalKeys method since that single key will be of known size. 

### Encoding the structs

We use the default golang var uint encoding. 
(Also used in amino) 
For referrence, this is encoding 7 bits per byte, with the leading bit specifying whether or not to read the next byte. 
(0 indicating not to read it) 
This encoding will be used for length prefixing, enum types, and weights. 

The pubkey for a `k of n` multisig should be encoded as `var_uint_encode(enum_type) || var_uint_encode(k) || var_uint_encode(n) || MarshalKeys(keys)`, where `keys` is a list of `n` pubkeys.

Marshal Keys should length prefix each pubkey\*, but other than that concatenate them all. In pseudocode:

```
MarshalKeys(keys [n]crypto.Pubkey) = var_uint_encode(len(keys[0])) || keys[0].Marshal || var_uint_encode(len(keys[1])) || keys[1].Marshal || ... || var_uint_encode(len(keys[n - 1])) || keys[n - 1].Marshal 
```

The weighted multi-sig pubkey will be encoded similarly.
Let `k` be the threshold, `weight_i` is the weighting of the `i`th key, and `keys` is a list of `n` pubkeys. 
Then the pubkey should be encoded as `var_uint_encode(enum_type) || var_uint_encode(k) || var_uint_encode(n) || var_uint_encode(weight_0) || ... || var_uint_encode(weight_{n-1}) ||  MarshalKeys(keys)` 

Before defining how the multisignature will be encoded, first we define the bitmap encoding. 
For `MarshalBitArray(indices []int, n int)`, creates a bitmap of size `⌈n / 8⌉` bytes. 
A `1` in any of the leading `n` bits of the bitmap indicates whether or not that pubkey is represented in the set. 

For a `k of n` multisig, there must be exactly `k` `1` bits. 
For a weighted multisig the sum of the weights of the assoc. keys must be greater than the threshold. 
There cannot be a single invalid signature, even if the sum of the correct ones is greater than the threshold.

Both types of multisignatures (weighted and unweighted) encode the same. 
It begins with the bitmap encoding of the indices.\*\* 
It will be followed by a list of `k` signatures, encoded with the same length prefixing and appending strategy as specified in `MarshalKeys`.\*\*\*

#### Footnotes

\* We cannot have it derive length from the pubkey enum type (proposed [here]((https://github.com/tendermint/go-crypto/issues/121)), to avoid length prefixing. This is because we want to support a `k` of `n` multisig of ASM's (this achieves different functionality than a weighted multi sig). 
ASM's don't have constant size pubkeys. 
The same log stands for length prefixing signatures.

\*\* The reason this doesn't use `log_8(n)` bytes per signer is because that heavily optimizes for the case where a very small number of signers are required.
e.g. for `n` of size `24`, that would only be more space efficient for `k < 3`. 
This seems less likely, and that it should not be the case optimized for.

\*\*\* Important decision: 
There is advocacy for using amino encoding of an `[][]byte`, instead of having a signature blob and parsing by reading the length prefix. (Note both are doing the same internally) 
Protobuf requires having a "field number" on each signature (this is a design choice in protobuf encoding of slices), which means an extra byte on all signatures relative to the proposed encoding scheme, in addition to a constant overhead.
This ADR does not propose using amino/protobuf for everything for the following reasons:
* Thats a large import for the crypto library, and using amino encoding doesn't save a significant amount of complexity for the end cryptography-library developer. 
* The proposed encoding scheme is sufficiently simple to implement. 
* The work involved in parsing the bitmap will likely take more work than marshalling the list of signatures.
* In general, we desire to remove the amino dependency in our crypto libraries, as we don't need upgradability on the level of each cryptosystem itself. ([Ref issue](https://github.com/tendermint/go-crypto/issues/121))
* Its important to note, in golang end developers won't notice the difference as we are just overriding the MarshalAmino methods.

Here are a few counterpoints to anticipated arguments for benefits of amino:

* People may complain as to why we aren't using "something standard" - There isn't a standard for encoding multisigs. Since we don't need upgradability of the individual signature types here, protobuf isn't a good choice. The advantage of the proposed scheme is that it saves space. 
 
* Its easier to just import protobuf then to make a custom parser - This is true, but consuming bytes is also quite easy (you can use your languages normal var uint parser, as the one we use is standard). This sort of raw marshalling of bytes is normal for things that have no need for upgradability. 

* The benefit of a custom encoding scheme isn't that significant - true, its `1` byte per signature, plus some constant overhead. However this will be on every multisig tx forever. Our cryptography should not have an extra field number byte per signature which doesn't provide anything, just because that is a decision choice protobuf3 had. There is a reason canonical encodings for cryptoschemes don't use encoding formats like protobuf - its less space efficient. Ideally we want this to become _the_ accountable subgroup multisig format, in addition to it being the accountable subgroup multisig format for the hub. 

### Future points of discussion

If desired, we can use ed25519 batch verification, or Davros batch verification for all ed25519 keys. This is a future point of discussion, but would be backwards compatible as this information won't need to be marshalled. (There may even be cofactor concerns without ristretto) Aggregation of pubkeys / sigs in Schnorr sigs / BLS sigs is not backwards compatible, and would need to be a new ASM type.

## Status

Proposed. We need a decision for using amino encoding. 
Once we have agreed upon this, we need to create a referrence implementation and test vectors.

## Consequences

### Positive
* Supports multisignatures, in a way that won't require any special cases in our downstream verification code. 

### Negative
* Larger codebase, however this should reside in a subfolder of tendermint/crypto, as it provides no new interfaces. (Ref #https://github.com/tendermint/go-crypto/issues/136)
* Another encoding type - Note this would be true for _any_ new pubkey addition

### Neutral
