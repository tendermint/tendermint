package types

const (
	PubKeyEd25519 = "ed25519"
)

func Ed25519ValidatorUpdate(pubkey []byte, power int64) ValidatorUpdate {
	return ValidatorUpdate{
		// Address:
		PubKey: PubKey{
			Type: PubKeyEd25519,
			Data: pubkey,
		},
		Power: power,
	}
}
