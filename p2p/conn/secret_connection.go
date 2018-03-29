// Uses nacl's secret_box to encrypt a net.Conn.
// It is (meant to be) an implementation of the STS protocol.
// Note we do not (yet) assume that a remote peer's pubkey
// is known ahead of time, and thus we are technically
// still vulnerable to MITM. (TODO!)
// See docs/sts-final.pdf for more info
package conn

import (
	"bytes"
	crand "crypto/rand"
	"crypto/sha256"
	"encoding/binary"
	"errors"
	"io"
	"net"
	"time"

	"golang.org/x/crypto/nacl/box"
	"golang.org/x/crypto/nacl/secretbox"
	"golang.org/x/crypto/ripemd160"

	crypto "github.com/tendermint/go-crypto"
	wire "github.com/tendermint/go-wire"
	cmn "github.com/tendermint/tmlibs/common"
)

// 2 + 1024 == 1026 total frame size
const dataLenSize = 2 // uint16 to describe the length, is <= dataMaxSize
const dataMaxSize = 1024
const totalFrameSize = dataMaxSize + dataLenSize
const sealedFrameSize = totalFrameSize + secretbox.Overhead
const authSigMsgSize = (32 + 1) + (64 + 1) // fixed size (length prefixed) byte arrays

// Implements net.Conn
type SecretConnection struct {
	conn       io.ReadWriteCloser
	recvBuffer []byte
	recvNonce  *[24]byte
	sendNonce  *[24]byte
	remPubKey  crypto.PubKey
	shrSecret  *[32]byte // shared secret
}

// Performs handshake and returns a new authenticated SecretConnection.
// Returns nil if error in handshake.
// Caller should call conn.Close()
// See docs/sts-final.pdf for more information.
func MakeSecretConnection(conn io.ReadWriteCloser, locPrivKey crypto.PrivKey) (*SecretConnection, error) {

	locPubKey := locPrivKey.PubKey()

	// Generate ephemeral keys for perfect forward secrecy.
	locEphPub, locEphPriv := genEphKeys()

	// Write local ephemeral pubkey and receive one too.
	// NOTE: every 32-byte string is accepted as a Curve25519 public key
	// (see DJB's Curve25519 paper: http://cr.yp.to/ecdh/curve25519-20060209.pdf)
	remEphPub, err := shareEphPubKey(conn, locEphPub)
	if err != nil {
		return nil, err
	}

	// Compute common shared secret.
	shrSecret := computeSharedSecret(remEphPub, locEphPriv)

	// Sort by lexical order.
	loEphPub, hiEphPub := sort32(locEphPub, remEphPub)

	// Check if the local ephemeral public key
	// was the least, lexicographically sorted.
	locIsLeast := bytes.Equal(locEphPub[:], loEphPub[:])

	// Generate nonces to use for secretbox.
	recvNonce, sendNonce := genNonces(loEphPub, hiEphPub, locIsLeast)

	// Generate common challenge to sign.
	challenge := genChallenge(loEphPub, hiEphPub)

	// Construct SecretConnection.
	sc := &SecretConnection{
		conn:       conn,
		recvBuffer: nil,
		recvNonce:  recvNonce,
		sendNonce:  sendNonce,
		shrSecret:  shrSecret,
	}

	// Sign the challenge bytes for authentication.
	locSignature := signChallenge(challenge, locPrivKey)

	// Share (in secret) each other's pubkey & challenge signature
	authSigMsg, err := shareAuthSignature(sc, locPubKey, locSignature)
	if err != nil {
		return nil, err
	}
	remPubKey, remSignature := authSigMsg.Key, authSigMsg.Sig
	if !remPubKey.VerifyBytes(challenge[:], remSignature) {
		return nil, errors.New("Challenge verification failed")
	}

	// We've authorized.
	sc.remPubKey = remPubKey
	return sc, nil
}

// Returns authenticated remote pubkey
func (sc *SecretConnection) RemotePubKey() crypto.PubKey {
	return sc.remPubKey
}

// Writes encrypted frames of `sealedFrameSize`
// CONTRACT: data smaller than dataMaxSize is read atomically.
func (sc *SecretConnection) Write(data []byte) (n int, err error) {
	for 0 < len(data) {
		var frame = make([]byte, totalFrameSize)
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
		var sealedFrame = make([]byte, sealedFrameSize)
		secretbox.Seal(sealedFrame[:0], frame, sc.sendNonce, sc.shrSecret)
		// fmt.Printf("secretbox.Seal(sealed:%X,sendNonce:%X,shrSecret:%X\n", sealedFrame, sc.sendNonce, sc.shrSecret)
		incr2Nonce(sc.sendNonce)
		// end encryption

		_, err := sc.conn.Write(sealedFrame)
		if err != nil {
			return n, err
		}
		n += len(chunk)
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

	sealedFrame := make([]byte, sealedFrameSize)
	_, err = io.ReadFull(sc.conn, sealedFrame)
	if err != nil {
		return
	}

	// decrypt the frame
	var frame = make([]byte, totalFrameSize)
	// fmt.Printf("secretbox.Open(sealed:%X,recvNonce:%X,shrSecret:%X\n", sealedFrame, sc.recvNonce, sc.shrSecret)
	_, ok := secretbox.Open(frame[:0], sealedFrame, sc.recvNonce, sc.shrSecret)
	if !ok {
		return n, errors.New("Failed to decrypt SecretConnection")
	}
	incr2Nonce(sc.recvNonce)
	// end decryption

	var chunkLength = binary.BigEndian.Uint16(frame) // read the first two bytes
	if chunkLength > dataMaxSize {
		return 0, errors.New("chunkLength is greater than dataMaxSize")
	}
	var chunk = frame[dataLenSize : dataLenSize+chunkLength]

	n = copy(data, chunk)
	sc.recvBuffer = chunk[n:]
	return
}

// Implements net.Conn
func (sc *SecretConnection) Close() error                  { return sc.conn.Close() }
func (sc *SecretConnection) LocalAddr() net.Addr           { return sc.conn.(net.Conn).LocalAddr() }
func (sc *SecretConnection) RemoteAddr() net.Addr          { return sc.conn.(net.Conn).RemoteAddr() }
func (sc *SecretConnection) SetDeadline(t time.Time) error { return sc.conn.(net.Conn).SetDeadline(t) }
func (sc *SecretConnection) SetReadDeadline(t time.Time) error {
	return sc.conn.(net.Conn).SetReadDeadline(t)
}
func (sc *SecretConnection) SetWriteDeadline(t time.Time) error {
	return sc.conn.(net.Conn).SetWriteDeadline(t)
}

func genEphKeys() (ephPub, ephPriv *[32]byte) {
	var err error
	ephPub, ephPriv, err = box.GenerateKey(crand.Reader)
	if err != nil {
		cmn.PanicCrisis("Could not generate ephemeral keypairs")
	}
	return
}

func shareEphPubKey(conn io.ReadWriteCloser, locEphPub *[32]byte) (remEphPub *[32]byte, err error) {
	// Send our pubkey and receive theirs in tandem.
	var trs, _ = cmn.Parallel(
		func(_ int) (val interface{}, err error, abort bool) {
			var _, err1 = conn.Write(locEphPub[:])
			if err1 != nil {
				return nil, err1, true // abort
			} else {
				return nil, nil, false
			}
		},
		func(_ int) (val interface{}, err error, abort bool) {
			var _remEphPub [32]byte
			var _, err2 = io.ReadFull(conn, _remEphPub[:])
			if err2 != nil {
				return nil, err2, true // abort
			} else {
				return _remEphPub, nil, false
			}
		},
	)

	// If error:
	if trs.FirstError() != nil {
		err = trs.FirstError()
		return
	}

	// Otherwise:
	var _remEphPub = trs.FirstValue().([32]byte)
	return &_remEphPub, nil
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
		recvNonce = nonce2
		sendNonce = nonce1
	}
	return
}

func genChallenge(loPubKey, hiPubKey *[32]byte) (challenge *[32]byte) {
	return hash32(append(loPubKey[:], hiPubKey[:]...))
}

func signChallenge(challenge *[32]byte, locPrivKey crypto.PrivKey) (signature crypto.Signature) {
	signature = locPrivKey.Sign(challenge[:])
	return
}

type authSigMessage struct {
	Key crypto.PubKey
	Sig crypto.Signature
}

func shareAuthSignature(sc *SecretConnection, pubKey crypto.PubKey, signature crypto.Signature) (recvMsg *authSigMessage, err error) {
	// Send our info and receive theirs in tandem.
	var trs, _ = cmn.Parallel(
		func(_ int) (val interface{}, err error, abort bool) {
			msgBytes := wire.BinaryBytes(authSigMessage{pubKey.Wrap(), signature.Wrap()})
			var _, err1 = sc.Write(msgBytes)
			if err1 != nil {
				return nil, err1, true // abort
			} else {
				return nil, nil, false
			}
		},
		func(_ int) (val interface{}, err error, abort bool) {
			readBuffer := make([]byte, authSigMsgSize)
			var _, err2 = io.ReadFull(sc, readBuffer)
			if err2 != nil {
				return nil, err2, true // abort
			}
			n := int(0) // not used.
			var _recvMsg = wire.ReadBinary(authSigMessage{}, bytes.NewBuffer(readBuffer), authSigMsgSize, &n, &err2).(authSigMessage)
			if err2 != nil {
				return nil, err2, true // abort
			} else {
				return _recvMsg, nil, false
			}
		},
	)

	// If error:
	if trs.FirstError() != nil {
		err = trs.FirstError()
		return
	}

	var _recvMsg = trs.FirstValue().(authSigMessage)
	return &_recvMsg, nil
}

//--------------------------------------------------------------------------------

// sha256
func hash32(input []byte) (res *[32]byte) {
	hasher := sha256.New()
	hasher.Write(input) // nolint: errcheck, gas
	resSlice := hasher.Sum(nil)
	res = new([32]byte)
	copy(res[:], resSlice)
	return
}

// We only fill in the first 20 bytes with ripemd160
func hash24(input []byte) (res *[24]byte) {
	hasher := ripemd160.New()
	hasher.Write(input) // nolint: errcheck, gas
	resSlice := hasher.Sum(nil)
	res = new([24]byte)
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
		nonce[i]++
		if nonce[i] != 0 {
			return
		}
	}
}
