package privval

import (
	"github.com/tendermint/tendermint/types"
)

type dashCoreSignerTestCase struct {
	chainID      string
	mockPV       types.PrivValidator
	signerClient *DashCoreSignerClient
	signerServer *SignerServer
}
