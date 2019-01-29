package wrapper

import secp256k1 "github.com/btcsuite/btcd/btcec"

type PublicKey secp256k1.PublicKey

func ParsePubKey(pubKeyStr []byte, curve *secp256k1.KoblitzCurve) (key *PublicKey, err error) {
	pub, err := secp256k1.ParsePubKey(pubKeyStr, curve)
	key = (*PublicKey)(pub)
	return
}
