package orderbook_test

import (
	"crypto/ed25519"
	"testing"

	"github.com/cosmos/gogoproto/proto"
	"github.com/stretchr/testify/require"
	dbm "github.com/tendermint/tm-db"

	"github.com/tendermint/tendermint/abci/example/orderbook"
	"github.com/tendermint/tendermint/abci/types"
	"github.com/tendermint/tendermint/crypto"
)

// TODO: we should also check that CheckTx adds bids and asks to the app-side mempool
func TestCheckTx(t *testing.T) {
	db := dbm.NewMemDB()
	require.NoError(t, orderbook.InitDB(db, []*orderbook.Pair{testPair}, nil))
	app, err := orderbook.New(db)
	require.NoError(t, err)

	testCases := []struct {
		name         string
		msg          *orderbook.Msg
		responseCode uint32
		expOrderSize int
	}{
		{
			name:         "test empty tx",
			msg:          &orderbook.Msg{},
			responseCode: orderbook.StatusErrValidateBasic,
			expOrderSize: 0,
		},
		{
			name: "test msg ask",
			msg: &orderbook.Msg{
				Sum: &orderbook.Msg_MsgAsk{
					MsgAsk: &orderbook.MsgAsk{
						Pair: testPair,
						AskOrder: &orderbook.OrderAsk{
							Quantity:  10,
							AskPrice:  1,
							OwnerId:   1,
							Signature: crypto.CRandBytes(ed25519.SignatureSize),
						},
					},
				},
			},
			responseCode: orderbook.StatusOK,
			expOrderSize: 1,
		},
		{
			name: "test msg ask wrong signature",
			msg: &orderbook.Msg{
				Sum: &orderbook.Msg_MsgAsk{
					MsgAsk: &orderbook.MsgAsk{
						Pair: testPair,
						AskOrder: &orderbook.OrderAsk{
							Quantity:  10,
							AskPrice:  1,
							OwnerId:   1,
							Signature: crypto.CRandBytes(62),
						},
					},
				},
			},
			responseCode: orderbook.StatusErrValidateBasic,
			expOrderSize: 1,
		},
		{
			name: "test msg bid",
			msg: &orderbook.Msg{Sum: &orderbook.Msg_MsgBid{MsgBid: &orderbook.MsgBid{
				Pair: testPair,
				BidOrder: &orderbook.OrderBid{
					MaxQuantity: 15,
					MaxPrice:    5,
					OwnerId:     1,
					Signature:   crypto.CRandBytes(ed25519.SignatureSize),
				},
			}}},
			responseCode: orderbook.StatusOK,
			expOrderSize: 2,
		},
		{
			name: "test msg bid blank",
			msg: &orderbook.Msg{Sum: &orderbook.Msg_MsgBid{MsgBid: &orderbook.MsgBid{
				Pair: testPair,
				BidOrder: &orderbook.OrderBid{
					MaxQuantity: 0,
					MaxPrice:    0,
					OwnerId:     0,
					Signature:   crypto.CRandBytes(ed25519.SignatureSize),
				},
			}}},
			responseCode: orderbook.StatusErrValidateBasic,
			expOrderSize: 2,
		},
		{
			name: "test msg register duplicate pair",
			msg: &orderbook.Msg{Sum: &orderbook.Msg_MsgRegisterPair{MsgRegisterPair: &orderbook.MsgRegisterPair{
				Pair: &orderbook.Pair{BuyersDenomination: "ATOM", SellersDenomination: "ATOM"},
			}}},
			responseCode: orderbook.StatusErrValidateBasic,
			expOrderSize: 2,
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			bz, err := proto.Marshal(tc.msg)
			require.NoError(t, err)
			resp := app.CheckTx(types.RequestCheckTx{Tx: bz})
			require.Equal(t, tc.responseCode, resp.Code, resp.Log)
			bids, asks := app.Orders(testPair)
			require.Equal(t, tc.expOrderSize, len(bids)+len(asks))
		})
	}
}

// func ValidateTx(t *testing.T) {
// 	db := dbm.NewMemDB()
// 	require.NoError(t, orderbook.InitDB(db, []*orderbook.Pair{testPair}, nil))
// 	app, err := orderbook.New(db)
// 	require.NoError(t, err)

// 	for _, tc := range testCases {
// 		t.Run(tc.name, func(t *testing.T) {
// 			bz, err := proto.Marshal(tc.msg)
// 			require.NoError(t, err)
// 			resp := app.CheckTx(types.RequestCheckTx{Tx: bz})
// 			require.Equal(t, tc.responseCode, resp.Code, resp.Log)
// 			bids, asks := app.Orders(testPair)
// 			require.Equal(t, tc.expOrderSize, len(bids)+len(asks))
// 		})
// 	}
// }

// TODO: we should check that transactions in
// a market are being validated and added to the proposal
// // and that other transactions get in
// func TestPrepareProposal(t *testing.T) {
// 	db := dbm.NewMemDB()
// 	require.NoError(t, orderbook.InitDB(db, []*orderbook.Pair{testPair}, nil))
// 	app, err := orderbook.New(db)
// 	require.NoError(t, err)

// 	for _, tc := range testCases {
// 		t.Run(tc.name, func(t *testing.T) {
// 			bz, err := proto.Marshal(tc.msg)
// 			require.NoError(t, err)
// 			resp := app.CheckTx(types.RequestCheckTx{Tx: bz})
// 			require.Equal(t, tc.responseCode, resp.Code, resp.Log)
// 			bids, asks := app.Orders(testPair)
// 			require.Equal(t, tc.expOrderSize, len(bids)+len(asks))
// 		})
// 	}
// }

// {
// 	name: "test msg register pair",
// 	msg: &orderbook.Msg{Sum: &orderbook.Msg_MsgRegisterPair{MsgRegisterPair: &orderbook.MsgRegisterPair{
// 		Pair: &orderbook.Pair{BuyersDenomination: "ATOM", SellersDenomination: "AUD"},
// 	}}},
// 	responseCode: orderbook.StatusOK,
// 	expOrderSize: 2,
// 	pairSize:     2,
// },

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

func asTxs(msgs ...*orderbook.Msg) [][]byte {
	output := make([][]byte, len(msgs))
	for i, msg := range msgs {
		bz, err := proto.Marshal(msg)
		if err != nil {
			panic(err)
		}
		output[i] = bz
	}
	return output
}

func newRegisterPair(d1, d2 string) *orderbook.Msg {
	return &orderbook.Msg{Sum: &orderbook.Msg_MsgRegisterPair{MsgRegisterPair: &orderbook.MsgRegisterPair{
		Pair: &orderbook.Pair{BuyersDenomination: d1, SellersDenomination: d2},
	}}}
}

func newRegisterAccount(pubkey []byte, commodities []*orderbook.Commodity ) *orderbook.Msg {
	return &orderbook.Msg{Sum: &orderbook.Msg_MsgCreateAccount{MsgCreateAccount: &orderbook.MsgCreateAccount{
		PublicKey: pubkey,
		Commodities: commodities,
	}}}
}

func TestEndToEnd(t *testing.T) {
	db := dbm.NewMemDB()
	_, err := orderbook.New(db)
	require.NoError(t, err)

	// registerPairMsg := newRegisterPair("NZD", "AUD")
	// registerAccountMsg := newRegisterAccount()

	// app.ProcessProposal(types.RequestProcessProposal{Txs: asTxs(registerPairMsg, registerAccountMsg)})

	// for _, tc := range testCases {
	// 	t.Run(tc.name, func(t *testing.T) {
	// 		bz, err := proto.Marshal(tc.msg)
	// 		require.NoError(t, err)
	// 		resp := app.DeliverTx(types.RequestDeliverTx{Tx: bz})
	// 		require.Equal(t, tc.responseCode, resp.Code, resp.Log)
	// 	})
	// }
	// 	name: "test create account",
	// 	msg: &orderbook.Msg{
	// 		Sum: &orderbook.Msg_MsgAsk{
	// 			MsgAsk: &orderbook.MsgAsk{
	// 				Pair: testPair,
	// 				AskOrder: &orderbook.OrderAsk{
	// 					Quantity:  10,
	// 					AskPrice:  1,
	// 					OwnerId:   1,
	// 					Signature: crypto.CRandBytes(ed25519.SignatureSize),
	// 				},
	// 			},
	// 		},
	// 	},
	// 	responseCode: orderbook.StatusOK,
	// 	expOrderSize: 1,
	// },
	// 		{
	// 	name: "test add tradeset",
	// 	msg: &orderbook.Msg{
	// 		Sum: &orderbook.Msg_MsgAsk{
	// 			MsgAsk: &orderbook.MsgAsk{
	// 				Pair: testPair,
	// 				AskOrder: &orderbook.OrderAsk{
	// 					Quantity:  10,
	// 					AskPrice:  1,
	// 					OwnerId:   1,
	// 					Signature: crypto.CRandBytes(ed25519.SignatureSize),
	// 				},
	// 			},
	// 		},
	// 	},
	// 	responseCode: orderbook.StatusOK,
	// 	expOrderSize: 1,
	// }

}
