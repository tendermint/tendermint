package orderbook_test

import (
	"testing"

	"github.com/cosmos/gogoproto/proto"
	"github.com/stretchr/testify/require"
	dbm "github.com/tendermint/tm-db"

	"github.com/tendermint/tendermint/abci/example/orderbook"
	"github.com/tendermint/tendermint/abci/types"
)

func TestCheckTx(t *testing.T) {
	app := orderbook.New(dbm.NewMemDB())

	testCases := []struct {
		name         string
		msg          *orderbook.Msg
		responseCode int
	}{
		{
			name:         "test empty tx",
			msg:          &orderbook.Msg{},
			responseCode: orderbook.ErrUnknownMessage,
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
				OrderBid: &orderbook.OrderBid{
					MaxQuantity: 15,
					MaxPrice:    5,
					OwnerId:     1,
					Signature:   []byte("signature"),
				},
			}},
			responseCode: orderbook.StatusOK,
		},
		{
			name: "test msg register pair",
			msg: &orderbook.Msg{Sum: &orderbook.Msg_MsgRegisterPair{MsgRegisterPair: &orderbook.MsgRegisterPair{
				Pair: testPair,
			}},
			responseCode: orderbook.StatusOK,
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

// func TestPrepareProposal(t *testing.T) {
// 	app := orderbook.New(dbm.NewMemDB())
// }

// func TestProcessProposal(t *testing.T) {
// 	app := orderbook.New(dbm.NewMemDB())
// }

// func TestFinalizeBlock(t *testing.T) {
// 	app := orderbook.New(dbm.NewMemDB())
// }
