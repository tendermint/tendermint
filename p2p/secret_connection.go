// Uses nacl's secret_box to encrypt a net.Conn.
package p2p

import (
	"bytes"
	crand "crypto/rand"
	"crypto/sha256"
	"encoding/binary"
	"errors"
	"io"
	"net"
	"sync"

	"golang.org/x/crypto/nacl/box"
	"golang.org/x/crypto/nacl/secretbox"
	"golang.org/x/crypto/ripemd160"

	acm "github.com/tendermint/tendermint/account"
	bm "github.com/tendermint/tendermint/binary"
)

// 2 + 1024 == 1026 total frame size
const dataLenSize = 2 // uint16 to describe the length, is <= dataMaxSize
const dataMaxSize = 1024
const totalFrameSize = dataMaxSize + dataLenSize

type SecretConnection struct {
	conn       net.Conn
	recvBuffer []byte
	recvNonce  *[24]byte
	sendNonce  *[24]byte
	remPubKey  acm.PubKeyEd25519
	shrSecret  *[32]byte // shared secret
}

// If handshake error, will return nil.
// Caller should call conn.Close()
func secretHandshake(conn net.Conn, locPrivKey acm.PrivKeyEd25519) (*SecretConnection, error) {

	locPubKey := locPrivKey.PubKey().(acm.PubKeyEd25519)

	// Generate ephemeral keys for perfect forward secrecy.
	locEphPub, locEphPriv := genEphKeys()

	// Write local ephemeral pubkey and receive one too.
	remEphPub, err := share32(conn, locEphPub)

	// Compute common shared secret.
	shrSecret := computeSharedSecret(remEphPub, locEphPriv)

	// Sort by lexical order.
	loEphPub, hiEphPub := sort32(locEphPub, remEphPub)

	// Generate nonces to use for secretbox.
	recvNonce, sendNonce := genNonces(loEphPub, hiEphPub, locEphPub == loEphPub)

	// Generate common challenge to sign.
	challenge := genChallenge(loEphPub, hiEphPub)

	// Construct SecretConnection.
	sc := &SecretConnection{
		conn:       conn,
		recvBuffer: nil,
		recvNonce:  recvNonce,
		sendNonce:  sendNonce,
		remPubKey:  nil,
		shrSecret:  shrSecret,
	}

	// Sign the challenge bytes for authentication.
	locSignature := signChallenge(challenge, locPrivKey)

	// Share (in secret) each other's pubkey & challenge signature
	remPubKey, remSignature, err := shareAuthSignature(sc, locPubKey, locSignature)
	if err != nil {
		return nil, err
	}
	if !remPubKey.VerifyBytes(challenge[:], remSignature) {
		return nil, errors.New("Challenge verification failed")
	}

	// We've authorized.
	sc.remPubKey = remPubKey
	return sc, nil
}

// CONTRACT: data smaller than dataMaxSize is read atomically.
func (sc *SecretConnection) Write(data []byte) (n int, err error) {
	for 0 < len(data) {
		var frame []byte = make([]byte, totalFrameSize)
		var chunk []byte
		if dataMaxSize < len(data) {
			chunk = data[:dataMaxSize]
			data = data[dataMaxSize:]
		} else {
			chunk = data
			data = nil
		}
		chunkLength := len(chunk)
		binary.BigEndian.PutUint16(frame, uint16(chunkLength))
		copy(frame[dataLenSize:], chunk)

		// encrypt the frame
		var sealedFrame = make([]byte, totalFrameSize+secretbox.Overhead)
		secretbox.Seal(sealedFrame, frame, sc.sendNonce, sc.shrSecret)
		incr2Nonce(sc.sendNonce)
		// end encryption

		_, err := sc.conn.Write(sealedFrame)
		if err != nil {
			return n, err
		} else {
			n += len(chunk)
		}
	}
	return
}

// CONTRACT: data smaller than dataMaxSize is read atomically.
func (sc *SecretConnection) Read(data []byte) (n int, err error) {
	if 0 < len(sc.recvBuffer) {
		n_ := copy(data, sc.recvBuffer)
		sc.recvBuffer = sc.recvBuffer[n_:]
		return
	}

	sealedFrame := make([]byte, totalFrameSize+secretbox.Overhead)
	_, err = io.ReadFull(sc.conn, sealedFrame)
	if err != nil {
		return
	}

	// decrypt the frame
	var frame = make([]byte, totalFrameSize)
	_, ok := secretbox.Open(frame, sealedFrame, sc.recvNonce, sc.shrSecret)
	if !ok {
		return n, errors.New("Failed to decrypt SecretConnection")
	}
	incr2Nonce(sc.recvNonce)
	// end decryption

	var chunkLength = binary.BigEndian.Uint16(frame)
	var chunk = frame[dataLenSize : dataLenSize+chunkLength]

	n = copy(data, chunk)
	sc.recvBuffer = chunk[n:]
	return
}

func genEphKeys() (ephPub, ephPriv *[32]byte) {
	var err error
	ephPub, ephPriv, err = box.GenerateKey(crand.Reader)
	if err != nil {
		panic("Could not generate ephemeral keypairs")
	}
	return
}

func share32(conn net.Conn, sendBytes *[32]byte) (recvBytes *[32]byte, err error) {
	var err1, err2 error
	var wg sync.WaitGroup
	wg.Add(2)

	go func() {
		defer wg.Done()
		_, err1 = conn.Write(sendBytes[:])
	}()

	go func() {
		defer wg.Done()
		recvBytes = new([32]byte)
		_, err2 = io.ReadFull(conn, recvBytes[:])
	}()

	wg.Wait()
	if err1 != nil {
		return nil, err1
	}
	if err2 != nil {
		return nil, err2
	}

	return recvBytes, nil
}

func computeSharedSecret(remPubKey, locPrivKey *[32]byte) (shrSecret *[32]byte) {
	shrSecret = new([32]byte)
	box.Precompute(shrSecret, remPubKey, locPrivKey)
	return
}

func sort32(foo, bar *[32]byte) (lo, hi *[32]byte) {
	if bytes.Compare(foo[:], bar[:]) < 0 {
		lo = foo
		hi = bar
	} else {
		lo = bar
		hi = foo
	}
	return
}

func genNonces(loPubKey, hiPubKey *[32]byte, locIsLo bool) (recvNonce, sendNonce *[24]byte) {
	nonce1 := hash24(append(loPubKey[:], hiPubKey[:]...))
	nonce2 := new([24]byte)
	copy(nonce2[:], nonce1[:])
	nonce2[len(nonce2)-1] ^= 0x01
	if locIsLo {
		recvNonce = nonce1
		sendNonce = nonce2
	} else {
		recvNonce = nonce1
		sendNonce = nonce2
	}
	return
}

func genChallenge(loPubKey, hiPubKey *[32]byte) (challenge *[32]byte) {
	return hash32(append(loPubKey[:], hiPubKey[:]...))
}

func signChallenge(challenge *[32]byte, locPrivKey acm.PrivKeyEd25519) (signature acm.SignatureEd25519) {
	signature = locPrivKey.Sign(challenge[:]).(acm.SignatureEd25519)
	return
}

type authSigMessage struct {
	Key acm.PubKeyEd25519
	Sig acm.SignatureEd25519
}

func shareAuthSignature(sc *SecretConnection, pubKey acm.PubKeyEd25519, signature acm.SignatureEd25519) (acm.PubKeyEd25519, acm.SignatureEd25519, error) {
	var recvMsg authSigMessage
	var err1, err2 error
	var wg sync.WaitGroup
	wg.Add(2)

	go func() {
		defer wg.Done()
		msgBytes := bm.BinaryBytes(authSigMessage{pubKey, signature})
		_, err1 = sc.Write(msgBytes)
	}()

	go func() {
		defer wg.Done()
		// NOTE relies on atomicity of small data.
		readBuffer := make([]byte, dataMaxSize)
		_, err2 = sc.Read(readBuffer)
		if err2 != nil {
			return
		}
		n := int64(0) // not used.
		recvMsg = bm.ReadBinary(authSigMessage{}, bytes.NewBuffer(readBuffer), &n, &err2).(authSigMessage)
	}()

	wg.Wait()
	if err1 != nil {
		return nil, nil, err1
	}
	if err2 != nil {
		return nil, nil, err2
	}

	return recvMsg.Key, recvMsg.Sig, nil
}

func verifyChallengeSignature(challenge *[32]byte, remPubKey acm.PubKeyEd25519, remSignature acm.SignatureEd25519) bool {
	return remPubKey.VerifyBytes(challenge[:], remSignature)
}

//--------------------------------------------------------------------------------

// sha256
func hash32(input []byte) (res *[32]byte) {
	hasher := sha256.New()
	hasher.Write(input) // does not error
	resSlice := hasher.Sum(nil)
	res = new([32]byte)
	copy(res[:], resSlice)
	return
}

// We only fill in the first 20 bytes with ripemd160
func hash24(input []byte) (res *[24]byte) {
	hasher := ripemd160.New()
	hasher.Write(input) // does not error
	resSlice := hasher.Sum(nil)
	res = new([24]byte)
	copy(res[:], resSlice)
	return
}

// ripemd160
func hash20(input []byte) (res *[20]byte) {
	hasher := ripemd160.New()
	hasher.Write(input) // does not error
	resSlice := hasher.Sum(nil)
	res = new([20]byte)
	copy(res[:], resSlice)
	return
}

// increment nonce big-endian by 2 with wraparound.
func incr2Nonce(nonce *[24]byte) {
	incrNonce(nonce)
	incrNonce(nonce)
}

// increment nonce big-endian by 1 with wraparound.
func incrNonce(nonce *[24]byte) {
	for i := 23; 0 <= i; i-- {
		nonce[i] += 1
		if nonce[i] != 0 {
			return
		}
	}
}
