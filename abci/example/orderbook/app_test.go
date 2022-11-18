package orderbook_test

import (
	"testing"

	"github.com/cosmos/gogoproto/proto"
	"github.com/stretchr/testify/require"
	dbm "github.com/tendermint/tm-db"

	"github.com/tendermint/tendermint/abci/example/orderbook"
	"github.com/tendermint/tendermint/abci/types"
)

// TODO: we should also check that CheckTx adds bids and asks to the app-side mempool
func TestCheckTx(t *testing.T) {
	app, err := orderbook.New(dbm.NewMemDB())
	require.NoError(t, err)

	testCases := []struct {
		name         string
		msg          *orderbook.Msg
		responseCode int
	}{
		{
			name:         "test empty tx",
			msg:          &orderbook.Msg{},
			responseCode: orderbook.StatusErrUnknownMessage,
		},
		{
			name: "test msg ask",
			msg: &orderbook.Msg{Sum: &orderbook.Msg_MsgAsk{MsgAsk: &orderbook.MsgAsk{
				Pair: testPair,
				AskOrder: &orderbook.OrderAsk{
					Quantity:  10,
					AskPrice:  1,
					OwnerId:   1,
					Signature: []byte("signature"),
				},
			}}},
			responseCode: orderbook.StatusOK,
		},
		{
			name: "test msg bid",
			msg: &orderbook.Msg{Sum: &orderbook.Msg_MsgBid{MsgBid: &orderbook.MsgBid{
				Pair: testPair,
				BidOrder: &orderbook.OrderBid{
					MaxQuantity: 15,
					MaxPrice:    5,
					OwnerId:     1,
					Signature:   []byte("signature"),
				},
			}}},
			responseCode: orderbook.StatusOK,
		},
		{
			name: "test msg register pair",
			msg: &orderbook.Msg{Sum: &orderbook.Msg_MsgRegisterPair{MsgRegisterPair: &orderbook.MsgRegisterPair{
				Pair: testPair,
			}}},
			responseCode: orderbook.StatusOK,
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			bz, err := proto.Marshal(tc.msg)
			require.NoError(t, err)
			resp := app.CheckTx(types.RequestCheckTx{Tx: bz})
			require.Equal(t, tc.responseCode, resp.Code)
		})
	}
}

// TODO: we should check that transactions in
// a market are being validated and added to the proposal
// and that other transactions get in
// func TestPrepareProposal(t *testing.T) {
// 	app := orderbook.New(dbm.NewMemDB())
// }

// TODO: we should test that transactions are
// always valid i.e. ValidateTx. We could potentially 
// combine this with PrepareProposal
// func TestProcessProposal(t *testing.T) {
// 	app := orderbook.New(dbm.NewMemDB())
// }

// TODO: we should test that a matched order
// correctly updates the accounts. We should
// also test that committing a block persists
// it to the database and that we can now
// query the new state
// func TestFinalizeBlock(t *testing.T) {
// 	app := orderbook.New(dbm.NewMemDB())
// }

// TODO: test that we can start from new
// and from existing state
// func TestNewStateMachine(t *testing.T) {}
