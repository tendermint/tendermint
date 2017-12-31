package types

import crypto "github.com/tendermint/go-crypto"

//-----------------------------------------------------------------

var _ Signer = (*DefaultSigner)(nil)

// DefaultSigner implements Signer.
// It uses a standard, unencrypted, in-memory crypto.PrivKey.
type DefaultSigner struct {
	PrivKey crypto.PrivKey `json:"priv_key"`
}

// NewDefaultSigner returns an instance of DefaultSigner.
func NewDefaultSigner(priv crypto.PrivKey) *DefaultSigner {
	return &DefaultSigner{
		PrivKey: priv,
	}
}

// Sign implements Signer. It signs the byte slice with a private key.
func (ds *DefaultSigner) Sign(msg []byte) (crypto.Signature, error) {
	return ds.PrivKey.Sign(msg), nil
}

//-----------------------------------------------------------------
