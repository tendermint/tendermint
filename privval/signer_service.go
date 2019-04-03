package privval

import (
	"fmt"

	"github.com/tendermint/tendermint/crypto"
	"github.com/tendermint/tendermint/types"
)

func HandleValidatorRequest(req RemoteSignerMsg, chainID string, privVal types.PrivValidator) (RemoteSignerMsg, error) {
	var res RemoteSignerMsg
	var err error

	switch r := req.(type) {
	case *PubKeyRequest:
		var p crypto.PubKey
		p = privVal.GetPubKey()
		res = &PubKeyResponse{p, nil}

	case *SignVoteRequest:
		err = privVal.SignVote(chainID, r.Vote)
		if err != nil {
			res = &SignedVoteResponse{nil, &RemoteSignerError{0, err.Error()}}
		} else {
			res = &SignedVoteResponse{r.Vote, nil}
		}

	case *SignProposalRequest:
		err = privVal.SignProposal(chainID, r.Proposal)
		if err != nil {
			res = &SignedProposalResponse{nil, &RemoteSignerError{0, err.Error()}}
		} else {
			res = &SignedProposalResponse{r.Proposal, nil}
		}

	default:
		err = fmt.Errorf("unknown msg: %v", r)
	}

	return res, err
}
