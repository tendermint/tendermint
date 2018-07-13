package merkle

import (
	"encoding/hex"
	"fmt"
	"net/url"
	"strings"

	cmn "github.com/tendermint/tmlibs/common"
)

/*

	For generalized Merkle proofs, each layer of the proof may require an
	optional key.  The key may be encoded either by URL-encoding or
	(upper-case) hex-encoding.
	TODO: In the future, more encodings may be supported, like base32 (e.g.
	/32:)

	For example, for a Cosmos-SDK application where the first two proof layers
	are SimpleValueOps, and the third proof layer is an IAVLValueOp, the keys
	might look like:

	0: []byte("App")
	1: []byte("IBC")
	2: []byte{0x01, 0x02, 0x03}

	Assuming that we know that the first two layers are always ASCII texts, we
	probably want to use URLEncoding for those, whereas the third layer will
	require HEX encoding for efficient representation.

	kp := new(KeyPath)
	kp.AppendKey([]byte("App"), KeyEncodingURL)
	kp.AppendKey([]byte("IBC"), KeyEncodingURL)
	kp.AppendKey([]byte{0x01, 0x02, 0x03}, KeyEncodingURL)
	kp.String() // Should return "/App/IBC/x:010203"

	NOTE: All encodings *MUST* work compatibly, such that you can choose to use
	whatever encoding, and the decoded keys will always be the same.  In other
	words, it's just as good to encode all three keys using URL encoding or HEX
	encoding... it just wouldn't be optimal in terms of readability or space
	efficiency.

	NOTE: Punycode will never be supported here, because not all values can be
	decoded.  For example, no string decodes to the string "xn--blah" in
	Punycode.

*/

type keyEncoding int

const (
	KeyEncodingURL keyEncoding = iota
	KeyEncodingHex
)

type KeyPath struct {
	keys [][]byte
	encs []keyEncoding
}

func (pth *KeyPath) AppendKey(key []byte, enc keyEncoding) {
	pth.keys = append(pth.keys, key)
	pth.encs = append(pth.encs, enc)
}

func (pth *KeyPath) String() string {
	res := ""
	for i := 0; i < len(pth.keys); i++ {
		key, enc := pth.keys[i], pth.encs[i]
		switch enc {
		case KeyEncodingURL:
			res += "/" + url.PathEscape(string(key))
		case KeyEncodingHex:
			res += "/x:" + fmt.Sprintf("%X", key)
		default:
			panic("unexpected key encoding type")
		}
	}
	return res
}

func KeyPathToKeys(path string) (keys [][]byte, err error) {
	if path == "" || path[0] != '/' {
		return nil, cmn.NewError("key path string must start with a forward slash '/'")
	}
	parts := strings.Split(path[1:], "/")
	keys = make([][]byte, len(parts))
	for i, part := range parts {
		if strings.HasPrefix(part, "x:") {
			hexPart := part[2:]
			key, err := hex.DecodeString(hexPart)
			if err != nil {
				return nil, cmn.ErrorWrap(err, "decoding hex-encoded part #%d: /%s", i, part)
			}
			keys[i] = key
		} else {
			key, err := url.PathUnescape(part)
			if err != nil {
				return nil, cmn.ErrorWrap(err, "decoding url-encoded part #%d: /%s", i, part)
			}
			keys[i] = []byte(key) // TODO Test this with random bytes, I'm not sure that it works for arbitrary bytes...
		}
	}
	return keys, nil
}
