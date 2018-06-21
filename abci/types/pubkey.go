package types

const (
	PubKeyEd25519 = "ed25519"
)

func Ed25519Validator(pubkey []byte, power int64) Validator {
	return Validator{
		// Address:
		PubKey: PubKey{
			Type: PubKeyEd25519,
			Data: pubkey,
		},
		Power: power,
	}
}
