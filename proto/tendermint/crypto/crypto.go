package crypto

// These functions export type tags for use with internal/jsontypes.

func (*PublicKey) TypeTag() string           { return "tendermint.crypto.PublicKey" }
func (*PublicKey_Ed25519) TypeTag() string   { return "tendermint.crypto.PublicKey_Ed25519" }
func (*PublicKey_Secp256K1) TypeTag() string { return "tendermint.crypto.PublicKey_Secp256K1" }
