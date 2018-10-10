package xsalsa20symmetric

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"golang.org/x/crypto/bcrypt" // forked to github.com/tendermint/crypto

	"github.com/tendermint/tendermint/crypto"
)

func TestSimple(t *testing.T) {

	crypto.MixEntropy([]byte("someentropy"))

	plaintext := []byte("sometext")
	secret := []byte("somesecretoflengththirtytwo===32")
	ciphertext := EncryptSymmetric(plaintext, secret)
	plaintext2, err := DecryptSymmetric(ciphertext, secret)

	require.Nil(t, err, "%+v", err)
	assert.Equal(t, plaintext, plaintext2)
}

func TestSimpleWithKDF(t *testing.T) {

	crypto.MixEntropy([]byte("someentropy"))

	plaintext := []byte("sometext")
	secretPass := []byte("somesecret")
	salt := []byte("somesaltsomesalt") // len 16
	// NOTE: we use a fork of x/crypto so we can inject our own randomness for salt
	secret, err := bcrypt.GenerateFromPassword(salt, secretPass, 12)
	if err != nil {
		t.Error(err)
	}
	secret = crypto.Sha256(secret)

	ciphertext := EncryptSymmetric(plaintext, secret)
	plaintext2, err := DecryptSymmetric(ciphertext, secret)

	require.Nil(t, err, "%+v", err)
	assert.Equal(t, plaintext, plaintext2)
}
