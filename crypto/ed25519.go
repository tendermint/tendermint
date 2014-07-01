package crypto

/*
#cgo CFLAGS: -g -m64 -O3 -fPIC -pthread
#cgo LDFLAGS: -lcrypto

#include <stdio.h>
#include "ed25519.h"
*/
import "C"
import "unsafe"

type Verify struct {
	Message   []byte
	PubKey    []byte
	Signature []byte
	Valid     bool
}

func MakePubKey(privKey []byte) []byte {
	pubKey := [32]byte{}
	C.ed25519_publickey(
		(*C.uchar)(unsafe.Pointer(&privKey[0])),
		(*C.uchar)(unsafe.Pointer(&pubKey[0])),
	)
	return pubKey[:]
}

func SignMessage(message []byte, privKey []byte, pubKey []byte) []byte {
	sig := [64]byte{}
	C.ed25519_sign(
		(*C.uchar)(unsafe.Pointer(&message[0])), (C.size_t)(len(message)),
		(*C.uchar)(unsafe.Pointer(&privKey[0])),
		(*C.uchar)(unsafe.Pointer(&pubKey[0])),
		(*C.uchar)(unsafe.Pointer(&sig[0])),
	)
	return sig[:]
}

func VerifyBatch(verifys []*Verify) bool {

	count := len(verifys)

	msgs := make([]*byte, count)
	lens := make([]C.size_t, count)
	pubs := make([]*byte, count)
	sigs := make([]*byte, count)
	valids := make([]C.int, count)

	for i, v := range verifys {
		msgs[i] = (*byte)(unsafe.Pointer(&v.Message[0]))
		lens[i] = (C.size_t)(len(v.Message))
		pubs[i] = (*byte)(&v.PubKey[0])
		sigs[i] = (*byte)(&v.Signature[0])
	}

	count_ := (C.size_t)(count)
	msgs_ := (**C.uchar)(unsafe.Pointer(&msgs[0]))
	lens_ := (*C.size_t)(unsafe.Pointer(&lens[0]))
	pubs_ := (**C.uchar)(unsafe.Pointer(&pubs[0]))
	sigs_ := (**C.uchar)(unsafe.Pointer(&sigs[0]))

	res := C.ed25519_sign_open_batch(msgs_, lens_, pubs_, sigs_, count_, &valids[0])

	for i, valid := range valids {
		verifys[i].Valid = valid > 0
	}

	return res == 0
}
