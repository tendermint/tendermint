package light_test

import (
	"testing"
	"time"

	// "github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	
	dbm "github.com/tendermint/tm-db"

	"github.com/tendermint/tendermint/libs/log"
	"github.com/tendermint/tendermint/light"
	"github.com/tendermint/tendermint/light/provider"
	mockp "github.com/tendermint/tendermint/light/provider/mock"
	dbs "github.com/tendermint/tendermint/light/store/db"
	// "github.com/tendermint/tendermint/proto/tendermint/types"
)

func TestDivergentTraces(t *testing.T) {
	primary := mockp.New(GenMockNode(chainID, 10, 5, 2, bTime))
	witness := mockp.New(GenMockNode(chainID, 10, 5, 2, bTime))
	
	c, err := light.NewClient(
		chainID,
		trustOptions,
		primary,
		[]provider.Provider{witness},
		dbs.New(dbm.NewMemDB(), chainID),
		light.Logger(log.TestingLogger()),
		light.MaxRetryAttempts(1),
	)
	require.NoError(t, err)
	
	_, err = c.VerifyLightBlockAtHeight(10, bTime.Add(1 * time.Hour))
	require.NoError(t, err)

}