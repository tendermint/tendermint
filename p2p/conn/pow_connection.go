// Perform a proof of work before connecting to peer
package conn

import (

	"crypto/sha256"
	crand "crypto/rand"
	"io"
	"net"
	"time"
	"errors"
	"fmt"
	cmn "github.com/tendermint/tmlibs/common"
)

// Proof of work connection
// Implements net.Conn
type PowConnection struct {
	conn io.ReadWriteCloser
}

// Makes both parties challenge one another for proofs of work, as means of spam
// mitigation.
func MakePowConnection(conn io.ReadWriteCloser, min_in_difficulty, max_out_difficulty int8) (*PowConnection, error) {
	localNonce := new([32]byte)
	crand.Read(localNonce[:])
	pc := &PowConnection{conn}
	localChal := makeChallenge(localNonce, min_in_difficulty)
	remoteChal, err := shareChallenges(conn, *localChal)
	if err != nil {
		return nil, err
	}

	remoteDifficulty, remoteNonce := getChallenge(remoteChal)
	if remoteDifficulty > max_out_difficulty {
		conn.Close()
		return nil, errors.New(fmt.Sprintf(
			"Peer asked for PoW w/ difficulty %d, max_out_difficulty is configured to %d",
			 remoteDifficulty,
			 max_out_difficulty,
			))
	}

	var trs, _ = cmn.Parallel(
		func(_ int) (val interface{}, err error, abort bool) {
			var _, err1 = cdc.MarshalBinaryWriter(conn, *createPoW(remoteDifficulty, remoteNonce))
			if err1 != nil {
				return nil, err1, true // abort
			} else {
				return nil, nil, false
			}
		},
		func(_ int) (val interface{}, err error, abort bool) {
			var _recvPoW = new([32]byte)
			var _, err2 = cdc.UnmarshalBinaryReader(conn, _recvPoW, 32)
			if err2 != nil {
				return nil, err2, true // abort
			} else {
				return _recvPoW, nil, false
			}
		},
	)

	if trs.FirstError() != nil {
		err := trs.FirstError()
		return nil, err
	}

	remotePoW := trs.FirstValue().([32]byte)

	if !validatePoW(min_in_difficulty, localNonce, &remotePoW) {
		conn.Close()
		return nil, errors.New(fmt.Sprintf(
			"Peer did not reply with a succesfull proof of work",
			))
	}
	// We've authenticated
	return pc, nil
}

// returns remote challenge
func shareChallenges(conn io.ReadWriteCloser, localChal [33]byte) ([33]byte, error) {
	var trs, _ = cmn.Parallel(
		func(_ int) (val interface{}, err error, abort bool) {
			var _, err1 = cdc.MarshalBinaryWriter(conn, localChal)
			if err1 != nil {
				return nil, err1, true // abort
			} else {
				return nil, nil, false
			}
		},
		func(_ int) (val interface{}, err error, abort bool) {
			var _recvMsg = new([33]byte)
			var _, err2 = cdc.UnmarshalBinaryReader(conn, _recvMsg, 35) // TODO
			if err2 != nil {
				return nil, err2, true // abort
			} else {
				return _recvMsg, nil, false
			}
		},
	)

	if trs.FirstError() != nil {
		err := trs.FirstError()
		// creates new byte arr, since nil is an invalid value there.
		return *new([33]byte), err
	}

	return trs.FirstValue().([33]byte), nil
}

func makeChallenge(nonce *[32]byte, minInputDifficulty int8) *[33]byte {
	chalSlice := append([]byte{byte(minInputDifficulty)}, nonce[:]...)
	chal := new([33]byte)
	copy(chal[:], chalSlice)
	return chal
}

func getChallenge(data [33]byte) ( int8, *[32]byte) {
	challengeDifficulty := int8(data[0])
	nonceSlice := (data[:])[1:33]
	nonce := new([32]byte)
	copy(nonce[:], nonceSlice)
	return challengeDifficulty, nonce
}

func createPoW(difficulty int8, nonce *[32]byte) (*[32]byte) {
	// Don't re-use sha256 state, see benchmarks
	// Can't start counter at 0, otherwise adversary may choose a nonce that
	// takes forever to validate on
	counter := new([32]byte)
	// Using crypto/rand, to avoid attacker choosing nonce adversarily based on
	// expected rand value from time
	crand.Read(counter[:])
	for {
		if validatePoW(difficulty, nonce, counter) {
			break
		}
		incrCounter(counter)
	}
	return counter
}

// Checks if PoW is valid
func validatePoW(difficulty int8, nonce, pow *[32]byte) (bool) {
	hash := sha256.New()
	_, err := hash.Write(nonce[:])
	if err != nil {
		return false
	}
	_, err = hash.Write(pow[:])
	if err != nil {
		return false
	}

	sum := hash.Sum(nil)
	return validatePoWHash(difficulty, sum)
}

func validatePoWHash(difficulty int8, powHash []byte) bool {
	// validate that the first N bits of the sum are 0, where N = difficulty
	// this checks it byte, by byte.
	for _, x := range powHash {
		for i := 0; i < 8; i++ {
			// efficiently check if leading bit is 0
			if (x<<1)>>1 == x {
				difficulty--
				x = x << 1
			} else {
				return false
			}

			if difficulty == 0 {
				return true
			}
		}
	}

	return false
}

// increment incrCounter big-endian by 1 with wraparound.
func incrCounter(ctr *[32]byte) {
	for i := 31; 0 <= i; i-- {
		ctr[i]++
		if ctr[i] != 0 {
			return
		}
	}
}

// Implements net.Conn
func (pc *PowConnection) Write(data []byte) (n int, err error) { return pc.Write(data) }
func (pc *PowConnection) Read(data []byte) (n int, err error)  { return pc.Read(data) }
func (pc *PowConnection) Close() error                         { return pc.conn.Close() }
func (pc *PowConnection) LocalAddr() net.Addr                  { return pc.conn.(net.Conn).LocalAddr() }
func (pc *PowConnection) RemoteAddr() net.Addr                 { return pc.conn.(net.Conn).RemoteAddr() }
func (pc *PowConnection) SetDeadline(t time.Time) error        { return pc.conn.(net.Conn).SetDeadline(t) }
func (pc *PowConnection) SetReadDeadline(t time.Time) error {
	return pc.conn.(net.Conn).SetReadDeadline(t)
}
func (pc *PowConnection) SetWriteDeadline(t time.Time) error {
	return pc.conn.(net.Conn).SetWriteDeadline(t)
}
