package proxy

import (
	"github.com/tendermint/tendermint/lite"
	lclient "github.com/tendermint/tendermint/lite/client"
	dbm "github.com/tendermint/tmlibs/db"
)

func GetCertifier(chainID, rootDir, nodeAddr string) (*lite.InquiringCertifier, error) {
	trust := lite.NewMultiProvider(
		lite.NewDBProvider(dbm.NewMemDB()),
		lite.NewDBProvider(dbm.NewDB("trust-base", dbm.LevelDBBackend, rootDir)),
	)

	source := lclient.NewHTTPProvider(nodeAddr)

	// XXX: total insecure hack to avoid `init`
	fc, err := source.LatestFullCommit(1, 1)
	if err != nil {
		return nil, err
	}

	cert, err := lite.NewInquiringCertifier(chainID, fc, trust, source)
	if err != nil {
		return nil, err
	}

	return cert, nil
}
