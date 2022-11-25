package orderbook_test

import (
	fmt "fmt"
	"testing"

	"github.com/cosmos/gogoproto/proto"
	"github.com/stretchr/testify/require"
	dbm "github.com/tendermint/tm-db"

	"github.com/tendermint/tendermint/abci/example/orderbook"
	"github.com/tendermint/tendermint/abci/types"
	"github.com/tendermint/tendermint/crypto"
	"github.com/tendermint/tendermint/crypto/ed25519"
	params "github.com/tendermint/tendermint/types"
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

func TestEndToEnd(t *testing.T) {
	db := dbm.NewMemDB()
	app, err := orderbook.New(db)
	require.NoError(t, err)
	

	var (
		maxBytes = params.DefaultConsensusParams().Block.MaxBytes
		commodityNZD    = &orderbook.Commodity{Denom: "NZD", Quantity: 100}
		commodityAUD    = &orderbook.Commodity{Denom: "AUD", Quantity: 100}
		registerPairMsg = newRegisterPair("NZD", "AUD")
		pair            = registerPairMsg.GetMsgRegisterPair().Pair
		pkAlice         = ed25519.GenPrivKey()
		pkBob           = ed25519.GenPrivKey()
		pubKeyAlice     = pkAlice.PubKey().Bytes()
		pubKeyBob       = pkBob.PubKey().Bytes()
		registerAlice   = newRegisterAccount(pubKeyAlice, []*orderbook.Commodity{commodityAUD})
		registerBob     = newRegisterAccount(pubKeyBob, []*orderbook.Commodity{commodityNZD})
		// bob is asking for 25 AUD for 5 NZD
		ask = &orderbook.Msg{Sum: &orderbook.Msg_MsgAsk{MsgAsk: orderbook.NewMsgAsk(pair, 5, 5, 1)}}
		// alice is bidding for 5 NZD for 25 AUD
		bid = &orderbook.Msg{Sum: &orderbook.Msg_MsgBid{MsgBid: orderbook.NewMsgBid(pair, 5, 5, 0)}}
	)

	require.NoError(t, ask.GetMsgAsk().Sign(pkBob))
	require.NoError(t, bid.GetMsgBid().Sign(pkAlice))

	testCases := []struct {
		txs      [][]byte
		accepted bool
		// assertions to be made about the state of the application
		// after each block
		assertions func(t *testing.T, app *orderbook.StateMachine)
	}{
		{
			// block 1 sets up the trading pair
			txs:      asTxs(registerPairMsg),
			accepted: true,
			assertions: func(t *testing.T, app *orderbook.StateMachine) {
				pairs := app.Pairs()
				require.Len(t, pairs, 1)
				require.Equal(t, pair, &pairs[0])
			},
		},
		{
			// block 2 registers two accounts: alice and bob
			txs:      asTxs(registerAlice, registerBob),
			accepted: true,
			assertions: func(t *testing.T, app *orderbook.StateMachine) {
				alice := app.Account(0)
				require.False(t, alice.IsEmpty(), alice)
				require.Equal(t, pubKeyAlice, alice.PublicKey)
				require.Len(t, alice.Commodities, 1)
				require.Equal(t, alice.Commodities[0], commodityAUD)
				bob := app.Account(1)
				require.False(t, bob.IsEmpty(), bob)
				require.Equal(t, pubKeyBob, bob.PublicKey)
				require.Len(t, bob.Commodities, 1)
				require.Equal(t, bob.Commodities[0], commodityNZD)
				require.True(t, app.Account(2).IsEmpty())
			},
		},
		{
			// block 3 performs a trade between alice and bob
			txs:      asTxs(ask, bid),
			accepted: true,
			assertions: func(t *testing.T, app *orderbook.StateMachine) {
				alice := app.Account(0)
				require.Equal(t, alice.Commodities[0].Quantity, 75) // 75 AUD
				require.Equal(t, alice.Commodities[1].Quantity, 5)  // 5 NZD
				bob := app.Account(1)
				require.Equal(t, bob.Commodities[0].Quantity, 95) // 95 NZD
				require.Equal(t, bob.Commodities[0].Quantity, 5)  // 5 AUD
			},
		},
	}

	for idx, tc := range testCases {
		for _, tx := range tc.txs {
			resp := app.CheckTx(types.RequestCheckTx{Tx: tx})
			require.EqualValues(t, orderbook.StatusOK, resp.Code)
		}
		txs := app.PrepareProposal(types.RequestPrepareProposal{MaxTxBytes: maxBytes, Txs: tc.txs}).Txs
		require.Equal(t, txs, tc.txs)
		if idx == 2 {
			fmt.Print(tc.txs)
			fmt.Println()
			fmt.Print(txs)
		}

		result := app.ProcessProposal(types.RequestProcessProposal{Txs: txs})
		if tc.accepted {
			require.Equal(t, types.ResponseProcessProposal_ACCEPT, result.Status)
		} else {
			require.Equal(t, types.ResponseProcessProposal_REJECT, result.Status)
			continue
		}

		app.BeginBlock(types.RequestBeginBlock{})
		for _, tx := range txs {
			app.DeliverTx(types.RequestDeliverTx{Tx: tx})
		}
		app.EndBlock(types.RequestEndBlock{})
		app.Commit()

		if tc.assertions != nil {
			tc.assertions(t, app)
		}
	}
}

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

func newRegisterAccount(pubkey []byte, commodities []*orderbook.Commodity) *orderbook.Msg {
	return &orderbook.Msg{Sum: &orderbook.Msg_MsgCreateAccount{MsgCreateAccount: &orderbook.MsgCreateAccount{
		PublicKey:   pubkey,
		Commodities: commodities,
	}}}

}
