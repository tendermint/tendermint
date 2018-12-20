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

	"golang.org/x/crypto/chacha20poly1305"
	"golang.org/x/crypto/curve25519"
	"golang.org/x/crypto/nacl/box"

	"github.com/tendermint/tendermint/crypto"
	cmn "github.com/tendermint/tendermint/libs/common"
	"golang.org/x/crypto/hkdf"
)

// 4 + 1024 == 1028 total frame size
const dataLenSize = 4
const dataMaxSize = 1024
const totalFrameSize = dataMaxSize + dataLenSize
const aeadSizeOverhead = 16 // overhead of poly 1305 authentication tag
const aeadKeySize = chacha20poly1305.KeySize
const aeadNonceSize = chacha20poly1305.NonceSize

// SecretConnection implements net.conn.
// It is an implementation of the STS protocol.
// Note we do not (yet) assume that a remote peer's pubkey
// is known ahead of time, and thus we are technically
// still vulnerable to MITM. (TODO!)
// See docs/sts-final.pdf for more info
type SecretConnection struct {
	conn       io.ReadWriteCloser
	recvBuffer []byte
	recvNonce  *[aeadNonceSize]byte
	sendNonce  *[aeadNonceSize]byte
	recvSecret *[aeadKeySize]byte
	sendSecret *[aeadKeySize]byte
	remPubKey  crypto.PubKey
}

// MakeSecretConnection performs handshake and returns a new authenticated
// SecretConnection.
// Returns nil if there is an error in handshake.
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

	// Sort by lexical order.
	loEphPub, _ := sort32(locEphPub, remEphPub)

	// Check if the local ephemeral public key
	// was the least, lexicographically sorted.
	locIsLeast := bytes.Equal(locEphPub[:], loEphPub[:])

	// Compute common diffie hellman secret using X25519.
	dhSecret := computeDHSecret(remEphPub, locEphPriv)

	// generate the secret used for receiving, sending, challenge via hkdf-sha2 on dhSecret
	recvSecret, sendSecret, challenge := deriveSecretAndChallenge(dhSecret, locIsLeast)

	// Construct SecretConnection.
	sc := &SecretConnection{
		conn:       conn,
		recvBuffer: nil,
		recvNonce:  new([aeadNonceSize]byte),
		sendNonce:  new([aeadNonceSize]byte),
		recvSecret: recvSecret,
		sendSecret: sendSecret,
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

// RemotePubKey returns authenticated remote pubkey
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
		binary.LittleEndian.PutUint32(frame, uint32(chunkLength))
		copy(frame[dataLenSize:], chunk)

		aead, err := chacha20poly1305.New(sc.sendSecret[:])
		if err != nil {
			return n, errors.New("Invalid SecretConnection Key")
		}
		// encrypt the frame
		var sealedFrame = make([]byte, aeadSizeOverhead+totalFrameSize)
		aead.Seal(sealedFrame[:0], sc.sendNonce[:], frame, nil)
		incrNonce(sc.sendNonce)
		// end encryption

		_, err = sc.conn.Write(sealedFrame)
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
		n = copy(data, sc.recvBuffer)
		sc.recvBuffer = sc.recvBuffer[n:]
		return
	}

	aead, err := chacha20poly1305.New(sc.recvSecret[:])
	if err != nil {
		return n, errors.New("Invalid SecretConnection Key")
	}
	sealedFrame := make([]byte, totalFrameSize+aeadSizeOverhead)
	_, err = io.ReadFull(sc.conn, sealedFrame)
	if err != nil {
		return
	}

	// decrypt the frame
	var frame = make([]byte, totalFrameSize)
	_, err = aead.Open(frame[:0], sc.recvNonce[:], sealedFrame, nil)
	if err != nil {
		return n, errors.New("Failed to decrypt SecretConnection")
	}
	incrNonce(sc.recvNonce)
	// end decryption

	var chunkLength = binary.LittleEndian.Uint32(frame) // read the first four bytes
	if chunkLength > dataMaxSize {
		return 0, errors.New("chunkLength is greater than dataMaxSize")
	}
	var chunk = frame[dataLenSize : dataLenSize+chunkLength]

	n = copy(data, chunk)
	sc.recvBuffer = chunk[n:]
	return
}

// Implements net.Conn
// nolint
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
		panic("Could not generate ephemeral keypairs")
	}
	return
}

func shareEphPubKey(conn io.ReadWriteCloser, locEphPub *[32]byte) (remEphPub *[32]byte, err error) {

	// Send our pubkey and receive theirs in tandem.
	var trs, _ = cmn.Parallel(
		func(_ int) (val interface{}, err error, abort bool) {
			var _, err1 = cdc.MarshalBinaryLengthPrefixedWriter(conn, locEphPub)
			if err1 != nil {
				return nil, err1, true // abort
			}
			return nil, nil, false
		},
		func(_ int) (val interface{}, err error, abort bool) {
			var _remEphPub [32]byte
			var _, err2 = cdc.UnmarshalBinaryLengthPrefixedReader(conn, &_remEphPub, 1024*1024) // TODO
			if err2 != nil {
				return nil, err2, true // abort
			}
			return _remEphPub, nil, false
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

func deriveSecretAndChallenge(dhSecret *[32]byte, locIsLeast bool) (recvSecret, sendSecret *[aeadKeySize]byte, challenge *[32]byte) {
	hash := sha256.New
	hkdf := hkdf.New(hash, dhSecret[:], nil, []byte("TENDERMINT_SECRET_CONNECTION_KEY_AND_CHALLENGE_GEN"))
	// get enough data for 2 aead keys, and a 32 byte challenge
	res := new([2*aeadKeySize + 32]byte)
	_, err := io.ReadFull(hkdf, res[:])
	if err != nil {
		panic(err)
	}

	challenge = new([32]byte)
	recvSecret = new([aeadKeySize]byte)
	sendSecret = new([aeadKeySize]byte)
	// Use the last 32 bytes as the challenge
	copy(challenge[:], res[2*aeadKeySize:2*aeadKeySize+32])

	// bytes 0 through aeadKeySize - 1 are one aead key.
	// bytes aeadKeySize through 2*aeadKeySize -1 are another aead key.
	// which key corresponds to sending and receiving key depends on whether
	// the local key is less than the remote key.
	if locIsLeast {
		copy(recvSecret[:], res[0:aeadKeySize])
		copy(sendSecret[:], res[aeadKeySize:aeadKeySize*2])
	} else {
		copy(sendSecret[:], res[0:aeadKeySize])
		copy(recvSecret[:], res[aeadKeySize:aeadKeySize*2])
	}

	return
}

func computeDHSecret(remPubKey, locPrivKey *[32]byte) (shrKey *[32]byte) {
	shrKey = new([32]byte)
	curve25519.ScalarMult(shrKey, locPrivKey, remPubKey)
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

func signChallenge(challenge *[32]byte, locPrivKey crypto.PrivKey) (signature []byte) {
	signature, err := locPrivKey.Sign(challenge[:])
	// TODO(ismail): let signChallenge return an error instead
	if err != nil {
		panic(err)
	}
	return
}

type authSigMessage struct {
	Key crypto.PubKey
	Sig []byte
}

func shareAuthSignature(sc *SecretConnection, pubKey crypto.PubKey, signature []byte) (recvMsg authSigMessage, err error) {

	// Send our info and receive theirs in tandem.
	var trs, _ = cmn.Parallel(
		func(_ int) (val interface{}, err error, abort bool) {
			var _, err1 = cdc.MarshalBinaryLengthPrefixedWriter(sc, authSigMessage{pubKey, signature})
			if err1 != nil {
				return nil, err1, true // abort
			}
			return nil, nil, false
		},
		func(_ int) (val interface{}, err error, abort bool) {
			var _recvMsg authSigMessage
			var _, err2 = cdc.UnmarshalBinaryLengthPrefixedReader(sc, &_recvMsg, 1024*1024) // TODO
			if err2 != nil {
				return nil, err2, true // abort
			}
			return _recvMsg, nil, false
		},
	)

	// If error:
	if trs.FirstError() != nil {
		err = trs.FirstError()
		return
	}

	var _recvMsg = trs.FirstValue().(authSigMessage)
	return _recvMsg, nil
}

//--------------------------------------------------------------------------------

// Increment nonce little-endian by 1 with wraparound.
// Due to chacha20poly1305 expecting a 12 byte nonce we do not use the first four
// bytes. We only increment a 64 bit unsigned int in the remaining 8 bytes
// (little-endian in nonce[4:]).
func incrNonce(nonce *[aeadNonceSize]byte) {
	counter := binary.LittleEndian.Uint64(nonce[4:])
	counter++
	binary.LittleEndian.PutUint64(nonce[4:], counter)
}
