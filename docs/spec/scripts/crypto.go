package main

import (
	"fmt"

	"github.com/tendermint/tendermint/crypto"
)

// SECRET
var SECRET = []byte("some secret")

func printEd() {
	priv := crypto.GenPrivKeyEd25519FromSecret(SECRET)
	pub := priv.PubKey().(crypto.PubKeyEd25519)
	sigV, err := priv.Sign([]byte("hello"))
	if err != nil {
		fmt.Println("Unexpected error:", err)
	}
	sig := sigV.(crypto.SignatureEd25519)

	name := "tendermint/PubKeyEd25519"
	length := len(pub[:])

	fmt.Println("### PubKeyEd25519")
	fmt.Println("")
	fmt.Println("```")
	fmt.Printf("// Name: %s\n", name)
	fmt.Printf("// PrefixBytes: 0x%X \n", pub.Bytes()[:4])
	fmt.Printf("// Length: 0x%X \n", length)
	fmt.Println("// Notes: raw 32-byte Ed25519 pubkey")
	fmt.Println("type PubKeyEd25519 [32]byte")
	fmt.Println("")
	fmt.Println(`func (pubkey PubKeyEd25519) Address() []byte {
  // NOTE: hash of the Amino encoded bytes!
  return RIPEMD160(AminoEncode(pubkey))
}`)
	fmt.Println("```")
	fmt.Println("")
	fmt.Printf("For example, the 32-byte Ed25519 pubkey `%X` would be encoded as `%X`.\n\n", pub[:], pub.Bytes())
	fmt.Printf("The address would then be `RIPEMD160(0x%X)` or `%X`\n", pub.Bytes(), pub.Address())
	fmt.Println("")

	name = "tendermint/SignatureKeyEd25519"
	length = len(sig[:])

	fmt.Println("### SignatureEd25519")
	fmt.Println("")
	fmt.Println("```")
	fmt.Printf("// Name: %s\n", name)
	fmt.Printf("// PrefixBytes: 0x%X \n", sig.Bytes()[:4])
	fmt.Printf("// Length: 0x%X \n", length)
	fmt.Println("// Notes: raw 64-byte Ed25519 signature")
	fmt.Println("type SignatureEd25519 [64]byte")
	fmt.Println("```")
	fmt.Println("")
	fmt.Printf("For example, the 64-byte Ed25519 signature `%X` would be encoded as `%X`\n", sig[:], sig.Bytes())
	fmt.Println("")

	name = "tendermint/PrivKeyEd25519"

	fmt.Println("### PrivKeyEd25519")
	fmt.Println("")
	fmt.Println("```")
	fmt.Println("// Name:", name)
	fmt.Println("// Notes: raw 32-byte priv key concatenated to raw 32-byte pub key")
	fmt.Println("type PrivKeyEd25519 [64]byte")
	fmt.Println("```")
}

func printSecp() {
	priv := crypto.GenPrivKeySecp256k1FromSecret(SECRET)
	pub := priv.PubKey().(crypto.PubKeySecp256k1)
	sigV, err := priv.Sign([]byte("hello"))
	if err != nil {
		fmt.Println("Unexpected error:", err)
	}
	sig := sigV.(crypto.SignatureSecp256k1)

	name := "tendermint/PubKeySecp256k1"
	length := len(pub[:])

	fmt.Println("### PubKeySecp256k1")
	fmt.Println("")
	fmt.Println("```")
	fmt.Printf("// Name: %s\n", name)
	fmt.Printf("// PrefixBytes: 0x%X \n", pub.Bytes()[:4])
	fmt.Printf("// Length: 0x%X \n", length)
	fmt.Println("// Notes: OpenSSL compressed pubkey prefixed with 0x02 or 0x03")
	fmt.Println("type PubKeySecp256k1 [33]byte")
	fmt.Println("")
	fmt.Println(`func (pubkey PubKeySecp256k1) Address() []byte {
  // NOTE: hash of the raw pubkey bytes (not Amino encoded!).
  // Compatible with Bitcoin addresses.
  return RIPEMD160(SHA256(pubkey[:]))
}`)
	fmt.Println("```")
	fmt.Println("")
	fmt.Printf("For example, the 33-byte Secp256k1 pubkey `%X` would be encoded as `%X`\n\n", pub[:], pub.Bytes())
	fmt.Printf("The address would then be `RIPEMD160(SHA256(0x%X))` or `%X`\n", pub[:], pub.Address())
	fmt.Println("")

	name = "tendermint/SignatureKeySecp256k1"

	fmt.Println("### SignatureSecp256k1")
	fmt.Println("")
	fmt.Println("```")
	fmt.Printf("// Name: %s\n", name)
	fmt.Printf("// PrefixBytes: 0x%X \n", sig.Bytes()[:4])
	fmt.Printf("// Length: Variable\n")
	fmt.Printf("// Encoding prefix: Variable\n")
	fmt.Println("// Notes: raw bytes of the Secp256k1 signature")
	fmt.Println("type SignatureSecp256k1 []byte")
	fmt.Println("```")
	fmt.Println("")
	fmt.Printf("For example, the Secp256k1 signature `%X` would be encoded as `%X`\n", []byte(sig[:]), sig.Bytes())
	fmt.Println("")

	name = "tendermint/PrivKeySecp256k1"

	fmt.Println("### PrivKeySecp256k1")
	fmt.Println("")
	fmt.Println("```")
	fmt.Println("// Name:", name)
	fmt.Println("// Notes: raw 32-byte priv key")
	fmt.Println("type PrivKeySecp256k1 [32]byte")
	fmt.Println("```")
}

func main() {
	printEd()
	fmt.Println("")
	printSecp()
}
