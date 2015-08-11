package state

import (
	"bytes"
	"encoding/hex"
	"fmt"
	"testing"

	tdb "github.com/tendermint/tendermint/db"
	ptypes "github.com/tendermint/tendermint/permission/types"
	. "github.com/tendermint/tendermint/state/types"
)

var chain_id = "lone_ranger"
var addr1, _ = hex.DecodeString("964B1493BBE3312278B7DEB94C39149F7899A345")
var send1, name1, call1 = 1, 1, 0
var perms, setbit = 66, 70
var accName = "me"
var roles1 = []string{"master", "universal-ruler"}
var amt1 int64 = 1000000
var g1 = fmt.Sprintf(`
{
    "chain_id":"%s",
    "accounts": [
        {
            "address": "%X",
            "amount": %d,
	    "name": "%s",
            "permissions": {
		    "base": {
			    "perms": %d,
			    "set": %d
		    },
            	    "roles": [
			"%s",
			"%s"
            	]
	    }
        }
    ],
    "validators": [
        {
            "amount": 100000000,
            "pub_key": [1,"F6C79CF0CB9D66B677988BCB9B8EADD9A091CD465A60542A8AB85476256DBA92"],
            "unbond_to": [
                {
                    "address": "964B1493BBE3312278B7DEB94C39149F7899A345",
                    "amount": 10000000
                }
            ]
        }
    ]
}
`, chain_id, addr1, amt1, accName, perms, setbit, roles1[0], roles1[1])

func TestGenesisReadable(t *testing.T) {
	genDoc := GenesisDocFromJSON([]byte(g1))
	if genDoc.ChainID != chain_id {
		t.Fatalf("Incorrect chain id. Got %d, expected %d\n", genDoc.ChainID, chain_id)
	}
	acc := genDoc.Accounts[0]
	if bytes.Compare(acc.Address, addr1) != 0 {
		t.Fatalf("Incorrect address for account. Got %X, expected %X\n", acc.Address, addr1)
	}
	if acc.Amount != amt1 {
		t.Fatalf("Incorrect amount for account. Got %d, expected %d\n", acc.Amount, amt1)
	}
	if acc.Name != accName {
		t.Fatalf("Incorrect name for account. Got %s, expected %s\n", acc.Name, accName)
	}

	perm, _ := acc.Permissions.Base.Get(ptypes.Send)
	if perm != (send1 > 0) {
		t.Fatalf("Incorrect permission for send. Got %v, expected %v\n", perm, send1 > 0)
	}
}

func TestGenesisMakeState(t *testing.T) {
	genDoc := GenesisDocFromJSON([]byte(g1))
	db := tdb.NewMemDB()
	st := MakeGenesisState(db, genDoc)
	acc := st.GetAccount(addr1)
	v, _ := acc.Permissions.Base.Get(ptypes.Send)
	if v != (send1 > 0) {
		t.Fatalf("Incorrect permission for send. Got %v, expected %v\n", v, send1 > 0)
	}
}
