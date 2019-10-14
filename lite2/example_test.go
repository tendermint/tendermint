package lite

import (
	"io/ioutil"
	"os"
	"testing"
	"time"

	"github.com/pkg/errors"

	dbm "github.com/tendermint/tm-db"

	httpp "github.com/tendermint/tendermint/lite2/provider/http"
	dbs "github.com/tendermint/tendermint/lite2/store/db"
	"github.com/tendermint/tendermint/types"
)

func TestExample(t *testing.T) {
	const (
		chainID = "my-awesome-chain"
	)
	dbDir, err := ioutil.TempDir("", "lite-client-example")
	if err != nil {
		t.Fatal(err)
	}
	defer os.RemoveAll(dbDir)

	// TODO: fetch the "trusted" header from a node
	header := (*types.SignedHeader)(nil)

	/////////////////////////////////////////////////////////////////////////////

	c, err := NewClient(
		chainID,
		TrustOptions{
			Period: 504 * time.Hour, // 21 days
			Height: 100,
			Hash:   header.Hash(),
		},
		httpp.New(chainID, "tcp://localhost:26657"),
		dbs.New(dbm.NewGoLevelDB("lite-client-db", dbDir), ""),
	)

	h, err := c.VerifyNextHeader()
	if err != nil {
		// retry?
	}
	// verify some data
}

func Test(t *testing.T) {

	const (
		chainID = "my-awesome-chain"
	)
	dbDir, err := ioutil.TempDir("", "lite-client-example")
	if err != nil {
		t.Fatal(err)
	}
	defer os.RemoveAll(dbDir)

	// TODO: fetch the "trusted" header from a node
	header := (*types.SignedHeader)(nil)

	/////////////////////////////////////////////////////////////////////////////

	base, err := NewClient(
		chainID,
		TrustOptions{
			Period: 504 * time.Hour, // 21 days
			Height: 100,
			Hash:   header.Hash(),
		},
		httpp.New(chainID, "tcp://localhost:26657"),
		dbs.New(dbm.NewGoLevelDB("lite-client-db", dbDir), ""),
	)

	c := NewAutoClient(base, 1*time.Second)
	defer c.Stop()

	select {
	case h := <-c.TrustedHeaders():
		// verify some data
	case err := <-c.Err():
		switch errors.Cause(err) {
		case ErrOldHeaderExpired:
			// reobtain trust height and hash
		default:
			// try with another full node
		}
	}
}
