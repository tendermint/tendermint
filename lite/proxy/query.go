package proxy

import (
	"fmt"

	cmn "github.com/tendermint/tmlibs/common"

	"github.com/tendermint/tendermint/crypto/merkle"
	"github.com/tendermint/tendermint/lite"
	rpcclient "github.com/tendermint/tendermint/rpc/client"
	ctypes "github.com/tendermint/tendermint/rpc/core/types"
	"github.com/tendermint/tendermint/types"
)

// GetWithProof will query the key on the given node, and verify it has
// a valid proof, as defined by the certifier.
//
// If there is any error in checking, returns an error.
func GetWithProof(key []byte, reqHeight int64, node rpcclient.Client,
	cert lite.Certifier) (
	val cmn.HexBytes, height int64, proof *merkle.Proof, err error) {

	if reqHeight < 0 {
		err = cmn.NewError("Height cannot be negative")
		return
	}

	res, err := GetWithProofOptions("/key", key,
		rpcclient.ABCIQueryOptions{Height: int64(reqHeight), Prove: true},
		node, cert)
	if err != nil {
		return
	}

	resp := res.Response
	val, height, proof = resp.Value, resp.Height, resp.Proof
	return val, height, proof, err
}

// GetWithProofOptions is useful if you want full access to the ABCIQueryOptions.
func GetWithProofOptions(path string, key []byte, opts rpcclient.ABCIQueryOptions,
	node rpcclient.Client, cert lite.Certifier) (
	*ctypes.ResultABCIQuery, error) {

	if !opts.Prove {
		return nil, cmn.NewError("require ABCIQueryOptions.Prove to be true")
	}

	res, err := node.ABCIQueryWithOptions(path, key, opts)
	if err != nil {
		return nil, err
	}
	resp := res.Response

	// Validate the response, e.g. height.
	if resp.IsErr() {
		err = cmn.NewError("Query error for key %d: %d", key, resp.Code)
		return nil, err
	}
	if len(resp.Key) == 0 || resp.Proof == nil {
		return nil, ErrNoData()
	}
	if resp.Height == 0 {
		return nil, cmn.NewError("Height returned is zero")
	}

	// AppHash for height H is in header H+1
	signedHeader, err := GetCertifiedCommit(resp.Height+1, node, cert)
	if err != nil {
		return nil, err
	}

	if len(resp.Value) > 0 {
		// Validate the proof against the certified header to ensure data integrity.
		// XXX Pass this in somehow so iavl can be registered.
		// XXX How do we encode the key into a string...
		prt := merkle.NewProofRuntime()
		err = prt.VerifyValue(resp.Proof, signedHeader.AppHash, resp.Value, string(resp.Key))
		if err != nil {
			return nil, cmn.ErrorWrap(err, "Couldn't verify proof")
		}
		return &ctypes.ResultABCIQuery{Response: resp}, nil
	} else {
		return nil, cmn.NewError("proof of absence not yet supported")
	}

	return &ctypes.ResultABCIQuery{Response: resp}, ErrNoData()
}

// GetCertifiedCommit gets the signed header for a given height and certifies
// it. Returns error if unable to get a proven header.
func GetCertifiedCommit(h int64, client rpcclient.Client, cert lite.Certifier) (types.SignedHeader, error) {

	// FIXME: cannot use cert.GetByHeight for now, as it also requires
	// Validators and will fail on querying tendermint for non-current height.
	// When this is supported, we should use it instead...
	rpcclient.WaitForHeight(client, h, nil)
	cresp, err := client.Commit(&h)
	if err != nil {
		return types.SignedHeader{}, err
	}

	// Validate downloaded checkpoint with our request and trust store.
	sh := cresp.SignedHeader
	if sh.Height != h {
		return types.SignedHeader{}, fmt.Errorf("height mismatch: want %v got %v",
			h, sh.Height)
	}

	if err = cert.Certify(sh); err != nil {
		return types.SignedHeader{}, err
	}

	return sh, nil
}
