package proxy

import (
	cmn "github.com/tendermint/tendermint/libs/common"

	"github.com/tendermint/tendermint/crypto/merkle"
	"github.com/tendermint/tendermint/lite"
	rpcclient "github.com/tendermint/tendermint/rpc/client"
	ctypes "github.com/tendermint/tendermint/rpc/core/types"
)

var _ rpcclient.Client = Wrapper{}

// Wrapper wraps a rpcclient with a Verifier and double-checks any input that is
// provable before passing it along. Allows you to make any rpcclient fully secure.
type Wrapper struct {
	rpcclient.Client
	cert *lite.DynamicVerifier
	prt  *merkle.ProofRuntime
}

// SecureClient uses a given Verifier to wrap an connection to an untrusted
// host and return a cryptographically secure rpc client.
//
// If it is wrapping an HTTP rpcclient, it will also wrap the websocket interface
func SecureClient(c rpcclient.Client, cert *lite.DynamicVerifier) Wrapper {
	prt := defaultProofRuntime()
	wrap := Wrapper{c, cert, prt}
	// TODO: no longer possible as no more such interface exposed....
	// if we wrap http client, then we can swap out the event switch to filter
	// if hc, ok := c.(*rpcclient.HTTP); ok {
	// 	evt := hc.WSEvents.EventSwitch
	// 	hc.WSEvents.EventSwitch = WrappedSwitch{evt, wrap}
	// }
	return wrap
}

// ABCIQueryWithOptions exposes all options for the ABCI query and verifies the returned proof
func (w Wrapper) ABCIQueryWithOptions(path string, data cmn.HexBytes,
	opts rpcclient.ABCIQueryOptions) (*ctypes.ResultABCIQuery, error) {

	res, err := GetWithProofOptions(w.prt, path, data, opts, w.Client, w.cert)
	return res, err
}

// ABCIQuery uses default options for the ABCI query and verifies the returned proof
func (w Wrapper) ABCIQuery(path string, data cmn.HexBytes) (*ctypes.ResultABCIQuery, error) {
	return w.ABCIQueryWithOptions(path, data, rpcclient.DefaultABCIQueryOptions)
}

// Tx queries for a given tx and verifies the proof if it was requested
func (w Wrapper) Tx(hash []byte, prove bool) (*ctypes.ResultTx, error) {
	res, err := w.Client.Tx(hash, prove)
	if !prove || err != nil {
		return res, err
	}
	h := int64(res.Height)
	sh, err := GetCertifiedCommit(h, w.Client, w.cert)
	if err != nil {
		return res, err
	}
	err = res.Proof.Validate(sh.DataHash)
	return res, err
}

// BlockchainInfo requests a list of headers and verifies them all...
// Rather expensive.
//
// TODO: optimize this if used for anything needing performance
func (w Wrapper) BlockchainInfo(minHeight, maxHeight int64) (*ctypes.ResultBlockchainInfo, error) {
	r, err := w.Client.BlockchainInfo(minHeight, maxHeight)
	if err != nil {
		return nil, err
	}

	// go and verify every blockmeta in the result....
	for _, meta := range r.BlockMetas {
		// get a checkpoint to verify from
		res, err := w.Commit(&meta.Header.Height)
		if err != nil {
			return nil, err
		}
		sh := res.SignedHeader
		err = ValidateBlockMeta(meta, sh)
		if err != nil {
			return nil, err
		}
	}

	return r, nil
}

// Block returns an entire block and verifies all signatures
func (w Wrapper) Block(height *int64) (*ctypes.ResultBlock, error) {
	resBlock, err := w.Client.Block(height)
	if err != nil {
		return nil, err
	}
	// get a checkpoint to verify from
	resCommit, err := w.Commit(height)
	if err != nil {
		return nil, err
	}
	sh := resCommit.SignedHeader

	// now verify
	err = ValidateBlockMeta(resBlock.BlockMeta, sh)
	if err != nil {
		return nil, err
	}
	err = ValidateBlock(resBlock.Block, sh)
	if err != nil {
		return nil, err
	}
	return resBlock, nil
}

// Commit downloads the Commit and certifies it with the lite.
//
// This is the foundation for all other verification in this module
func (w Wrapper) Commit(height *int64) (*ctypes.ResultCommit, error) {
	if height == nil {
		resStatus, err := w.Client.Status()
		if err != nil {
			return nil, err
		}
		// NOTE: If resStatus.CatchingUp, there is a race
		// condition where the validator set for the next height
		// isn't available until some time after the blockstore
		// has height h on the remote node.  This isn't an issue
		// once the node has caught up, and a syncing node likely
		// won't have this issue esp with the implementation we
		// have here, but we may have to address this at some
		// point.
		height = new(int64)
		*height = resStatus.SyncInfo.LatestBlockHeight
	}
	rpcclient.WaitForHeight(w.Client, *height, nil)
	res, err := w.Client.Commit(height)
	// if we got it, then verify it
	if err == nil {
		sh := res.SignedHeader
		err = w.cert.Verify(sh)
	}
	return res, err
}

// // WrappedSwitch creates a websocket connection that auto-verifies any info
// // coming through before passing it along.
// //
// // Since the verification takes 1-2 rpc calls, this is obviously only for
// // relatively low-throughput situations that can tolerate a bit extra latency
// type WrappedSwitch struct {
// 	types.EventSwitch
// 	client rpcclient.Client
// }

// // FireEvent verifies any block or header returned from the eventswitch
// func (s WrappedSwitch) FireEvent(event string, data events.EventData) {
// 	tm, ok := data.(types.TMEventData)
// 	if !ok {
// 		fmt.Printf("bad type %#v\n", data)
// 		return
// 	}

// 	// check to validate it if possible, and drop if not valid
// 	switch t := tm.(type) {
// 	case types.EventDataNewBlockHeader:
// 		err := verifyHeader(s.client, t.Header)
// 		if err != nil {
// 			fmt.Printf("Invalid header: %#v\n", err)
// 			return
// 		}
// 	case types.EventDataNewBlock:
// 		err := verifyBlock(s.client, t.Block)
// 		if err != nil {
// 			fmt.Printf("Invalid block: %#v\n", err)
// 			return
// 		}
// 		// TODO: can we verify tx as well? anything else
// 	}

// 	// looks good, we fire it
// 	s.EventSwitch.FireEvent(event, data)
// }

// func verifyHeader(c rpcclient.Client, head *types.Header) error {
// 	// get a checkpoint to verify from
// 	commit, err := c.Commit(&head.Height)
// 	if err != nil {
// 		return err
// 	}
// 	check := certclient.CommitFromResult(commit)
// 	return ValidateHeader(head, check)
// }
//
// func verifyBlock(c rpcclient.Client, block *types.Block) error {
// 	// get a checkpoint to verify from
// 	commit, err := c.Commit(&block.Height)
// 	if err != nil {
// 		return err
// 	}
// 	check := certclient.CommitFromResult(commit)
// 	return ValidateBlock(block, check)
// }
