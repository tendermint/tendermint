package conn

import (
	"bytes"
	"crypto/cipher"
	crand "crypto/rand"
	"crypto/sha256"
	"encoding/binary"
	"io"
	"math"
	"net"
	"sync"
	"time"

	"github.com/gtank/merlin"
	pool "github.com/libp2p/go-buffer-pool"
	"github.com/pkg/errors"
	"golang.org/x/crypto/chacha20poly1305"
	"golang.org/x/crypto/curve25519"
	"golang.org/x/crypto/hkdf"
	"golang.org/x/crypto/nacl/box"

	"github.com/tendermint/tendermint/crypto"
	"github.com/tendermint/tendermint/crypto/ed25519"
	"github.com/tendermint/tendermint/libs/async"
)

// 4 + 1024 == 1028 total frame size
const (
	dataLenSize      = 4
	dataMaxSize      = 1024
	totalFrameSize   = dataMaxSize + dataLenSize
	aeadSizeOverhead = 16 // overhead of poly 1305 authentication tag
	aeadKeySize      = chacha20poly1305.KeySize
	aeadNonceSize    = chacha20poly1305.NonceSize
)

var (
	ErrSmallOrderRemotePubKey = errors.New("detected low order point from remote peer")

	labelEphemeralLowerPublicKey = []byte("EPHEMERAL_LOWER_PUBLIC_KEY")
	labelEphemeralUpperPublicKey = []byte("EPHEMERAL_UPPER_PUBLIC_KEY")
	labelDHSecret                = []byte("DH_SECRET")
	labelSecretConnectionMac     = []byte("SECRET_CONNECTION_MAC")

	secretConnKeyAndChallengeGen = []byte("TENDERMINT_SECRET_CONNECTION_KEY_AND_CHALLENGE_GEN")
)

// SecretConnection implements net.Conn.
// It is an implementation of the STS protocol.
// See https://github.com/tendermint/tendermint/blob/0.1/docs/sts-final.pdf for
// details on the protocol.
//
// Consumers of the SecretConnection are responsible for authenticating
// the remote peer's pubkey against known information, like a nodeID.
// Otherwise they are vulnerable to MITM.
// (TODO(ismail): see also https://github.com/tendermint/tendermint/issues/3010)
type SecretConnection struct {

	// immutable
	recvAead cipher.AEAD
	sendAead cipher.AEAD

	remPubKey crypto.PubKey
	conn      io.ReadWriteCloser

	// net.Conn must be thread safe:
	// https://golang.org/pkg/net/#Conn.
	// Since we have internal mutable state,
	// we need mtxs. But recv and send states
	// are independent, so we can use two mtxs.
	// All .Read are covered by recvMtx,
	// all .Write are covered by sendMtx.
	recvMtx    sync.Mutex
	recvBuffer []byte
	recvNonce  *[aeadNonceSize]byte

	sendMtx   sync.Mutex
	sendNonce *[aeadNonceSize]byte
}

// MakeSecretConnection performs handshake and returns a new authenticated
// SecretConnection.
// Returns nil if there is an error in handshake.
// Caller should call conn.Close()
// See docs/sts-final.pdf for more information.
func MakeSecretConnection(conn io.ReadWriteCloser, locPrivKey crypto.PrivKey) (*SecretConnection, error) {
	var (
		locPubKey = locPrivKey.PubKey()
	)

	// Generate ephemeral keys for perfect forward secrecy.
	locEphPub, locEphPriv := genEphKeys()

	// Write local ephemeral pubkey and receive one too.
	// NOTE: every 32-byte string is accepted as a Curve25519 public key (see
	// DJB's Curve25519 paper: http://cr.yp.to/ecdh/curve25519-20060209.pdf)
	remEphPub, err := shareEphPubKey(conn, locEphPub)
	if err != nil {
		return nil, err
	}

	// Sort by lexical order.
	loEphPub, hiEphPub := sort32(locEphPub, remEphPub)

	transcript := merlin.NewTranscript("TENDERMINT_SECRET_CONNECTION_TRANSCRIPT_HASH")

	transcript.AppendMessage(labelEphemeralLowerPublicKey, loEphPub[:])
	transcript.AppendMessage(labelEphemeralUpperPublicKey, hiEphPub[:])

	// Check if the local ephemeral public key was the least, lexicographically
	// sorted.
	locIsLeast := bytes.Equal(locEphPub[:], loEphPub[:])

	// Compute common diffie hellman secret using X25519.
	dhSecret, err := computeDHSecret(remEphPub, locEphPriv)
	if err != nil {
		return nil, err
	}

	transcript.AppendMessage(labelDHSecret, dhSecret[:])

	// Generate the secret used for receiving, sending, challenge via HKDF-SHA2
	// on the transcript state (which itself also uses HKDF-SHA2 to derive a key
	// from the dhSecret).
	recvSecret, sendSecret := deriveSecrets(dhSecret, locIsLeast)

	const challengeSize = 32
	var challenge [challengeSize]byte
	challengeSlice := transcript.ExtractBytes(labelSecretConnectionMac, challengeSize)

	copy(challenge[:], challengeSlice[0:challengeSize])

	sendAead, err := chacha20poly1305.New(sendSecret[:])
	if err != nil {
		return nil, errors.New("invalid send SecretConnection Key")
	}
	recvAead, err := chacha20poly1305.New(recvSecret[:])
	if err != nil {
		return nil, errors.New("invalid receive SecretConnection Key")
	}

	sc := &SecretConnection{
		conn:       conn,
		recvBuffer: nil,
		recvNonce:  new([aeadNonceSize]byte),
		sendNonce:  new([aeadNonceSize]byte),
		recvAead:   recvAead,
		sendAead:   sendAead,
	}

	// Sign the challenge bytes for authentication.
	locSignature := signChallenge(&challenge, locPrivKey)

	// Share (in secret) each other's pubkey & challenge signature
	authSigMsg, err := shareAuthSignature(sc, locPubKey, locSignature)
	if err != nil {
		return nil, err
	}

	remPubKey, remSignature := authSigMsg.Key, authSigMsg.Sig
	if _, ok := remPubKey.(ed25519.PubKeyEd25519); !ok {
		return nil, errors.Errorf("expected ed25519 pubkey, got %T", remPubKey)
	}
	if !remPubKey.VerifyBytes(challenge[:], remSignature) {
		return nil, errors.New("challenge verification failed")
	}

	// We've authorized.
	sc.remPubKey = remPubKey
	return sc, nil
}

// RemotePubKey returns authenticated remote pubkey
func (sc *SecretConnection) RemotePubKey() crypto.PubKey {
	return sc.remPubKey
}

// Writes encrypted frames of `totalFrameSize + aeadSizeOverhead`.
// CONTRACT: data smaller than dataMaxSize is written atomically.
func (sc *SecretConnection) Write(data []byte) (n int, err error) {
	sc.sendMtx.Lock()
	defer sc.sendMtx.Unlock()

	for 0 < len(data) {
		if err := func() error {
			var sealedFrame = pool.Get(aeadSizeOverhead + totalFrameSize)
			var frame = pool.Get(totalFrameSize)
			defer func() {
				pool.Put(sealedFrame)
				pool.Put(frame)
			}()
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

			// encrypt the frame
			sc.sendAead.Seal(sealedFrame[:0], sc.sendNonce[:], frame, nil)
			incrNonce(sc.sendNonce)
			// end encryption

			_, err = sc.conn.Write(sealedFrame)
			if err != nil {
				return err
			}
			n += len(chunk)
			return nil
		}(); err != nil {
			return n, err
		}
	}
	return n, err
}

// CONTRACT: data smaller than dataMaxSize is read atomically.
func (sc *SecretConnection) Read(data []byte) (n int, err error) {
	sc.recvMtx.Lock()
	defer sc.recvMtx.Unlock()

	// read off and update the recvBuffer, if non-empty
	if 0 < len(sc.recvBuffer) {
		n = copy(data, sc.recvBuffer)
		sc.recvBuffer = sc.recvBuffer[n:]
		return
	}

	// read off the conn
	var sealedFrame = pool.Get(aeadSizeOverhead + totalFrameSize)
	defer pool.Put(sealedFrame)
	_, err = io.ReadFull(sc.conn, sealedFrame)
	if err != nil {
		return
	}

	// decrypt the frame.
	// reads and updates the sc.recvNonce
	var frame = pool.Get(totalFrameSize)
	defer pool.Put(frame)
	_, err = sc.recvAead.Open(frame[:0], sc.recvNonce[:], sealedFrame, nil)
	if err != nil {
		return n, errors.New("failed to decrypt SecretConnection")
	}
	incrNonce(sc.recvNonce)
	// end decryption

	// copy checkLength worth into data,
	// set recvBuffer to the rest.
	var chunkLength = binary.LittleEndian.Uint32(frame) // read the first four bytes
	if chunkLength > dataMaxSize {
		return 0, errors.New("chunkLength is greater than dataMaxSize")
	}
	var chunk = frame[dataLenSize : dataLenSize+chunkLength]
	n = copy(data, chunk)
	if n < len(chunk) {
		sc.recvBuffer = make([]byte, len(chunk)-n)
		copy(sc.recvBuffer, chunk[n:])
	}
	return n, err
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
	// TODO: Probably not a problem but ask Tony: different from the rust implementation (uses x25519-dalek),
	// we do not "clamp" the private key scalar:
	// see: https://github.com/dalek-cryptography/x25519-dalek/blob/34676d336049df2bba763cc076a75e47ae1f170f/src/x25519.rs#L56-L74
	ephPub, ephPriv, err = box.GenerateKey(crand.Reader)
	if err != nil {
		panic("Could not generate ephemeral key-pair")
	}
	return
}

func shareEphPubKey(conn io.ReadWriter, locEphPub *[32]byte) (remEphPub *[32]byte, err error) {

	// Send our pubkey and receive theirs in tandem.
	var trs, _ = async.Parallel(
		func(_ int) (val interface{}, abort bool, err error) {
			var _, err1 = cdc.MarshalBinaryLengthPrefixedWriter(conn, locEphPub)
			if err1 != nil {
				return nil, true, err1 // abort
			}
			return nil, false, nil
		},
		func(_ int) (val interface{}, abort bool, err error) {
			var _remEphPub [32]byte
			var _, err2 = cdc.UnmarshalBinaryLengthPrefixedReader(conn, &_remEphPub, 1024*1024) // TODO
			if err2 != nil {
				return nil, true, err2 // abort
			}
			return _remEphPub, false, nil
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

func deriveSecrets(
	dhSecret *[32]byte,
	locIsLeast bool,
) (recvSecret, sendSecret *[aeadKeySize]byte) {
	hash := sha256.New
	hkdf := hkdf.New(hash, dhSecret[:], nil, secretConnKeyAndChallengeGen)
	// get enough data for 2 aead keys, and a 32 byte challenge
	res := new([2*aeadKeySize + 32]byte)
	_, err := io.ReadFull(hkdf, res[:])
	if err != nil {
		panic(err)
	}

	recvSecret = new([aeadKeySize]byte)
	sendSecret = new([aeadKeySize]byte)

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

// computeDHSecret computes a Diffie-Hellman shared secret key
// from our own local private key and the other's public key.
func computeDHSecret(remPubKey, locPrivKey *[32]byte) (*[32]byte, error) {
	shrKey, err := curve25519.X25519(locPrivKey[:], remPubKey[:])
	if err != nil {
		return nil, err
	}
	var shrKeyArray [32]byte
	copy(shrKeyArray[:], shrKey)
	return &shrKeyArray, nil
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

func shareAuthSignature(sc io.ReadWriter, pubKey crypto.PubKey, signature []byte) (recvMsg authSigMessage, err error) {

	// Send our info and receive theirs in tandem.
	var trs, _ = async.Parallel(
		func(_ int) (val interface{}, abort bool, err error) {
			var _, err1 = cdc.MarshalBinaryLengthPrefixedWriter(sc, authSigMessage{pubKey, signature})
			if err1 != nil {
				return nil, true, err1 // abort
			}
			return nil, false, nil
		},
		func(_ int) (val interface{}, abort bool, err error) {
			var _recvMsg authSigMessage
			var _, err2 = cdc.UnmarshalBinaryLengthPrefixedReader(sc, &_recvMsg, 1024*1024) // TODO
			if err2 != nil {
				return nil, true, err2 // abort
			}
			return _recvMsg, false, nil
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
	if counter == math.MaxUint64 {
		// Terminates the session and makes sure the nonce would not re-used.
		// See https://github.com/tendermint/tendermint/issues/3531
		panic("can't increase nonce without overflow")
	}
	counter++
	binary.LittleEndian.PutUint64(nonce[4:], counter)
}
