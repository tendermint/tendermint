//go:generate go install google.golang.org/protobuf/cmd/protoc-gen-go@latest
//go:generate protoc -I. -I../.. --go_out=. --go_opt=paths=source_relative wire.proto msgs.proto

// The orderbook presents a more advanced example of a Tendermint application than the simple kvstore
//
// An orderbook is a tool used in financial markets for enabling trading of various commodities. Without
// delving into too much detail, an orderbook is made of two types of transactions: Bids and Asks. An Ask
// is an offer by a seller for n amount of a commodity at an AskPrice and a bid is an offer from a buyer
// for m amount of a commodity at a BidPrice. When the bid price exceeds the ask price, and the buyer quantity
// is less than or equal to the sellers quantity, the order is matched. In actual terms, we neglect the
// underlying denomination (i.e. USD) and effectively both participants are simultaneously a buyer and seller.
//
// This example falls far short of being a decentralized orderbook, but demonstrates how one can build an
// app-side mempool, how one can use PrepareProposal and ProcessProposal to craft complex transactions,
// how we can use signatures and validate transactions against state. How applications can manage concurrency,
// and demonstrate the lifecycle of transactions from RPC -> CheckTx -> Mempool -> PrepareProposal -> ProcessProposal
// -> DeliverTx -> Commit -> Querying

package orderbook
