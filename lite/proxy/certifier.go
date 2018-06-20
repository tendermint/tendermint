package proxy

import (
	"github.com/tendermint/tendermint/lite"
	lclient "github.com/tendermint/tendermint/lite/client"
	dbm "github.com/tendermint/tmlibs/db"
)

func GetCertifier(chainID, rootDir string, client lclient.SignStatusClient) (*lite.InquiringCertifier, error) {
	trust := lite.NewMultiProvider(
		lite.NewDBProvider(dbm.NewMemDB()).SetLimit(10),
		lite.NewDBProvider(dbm.NewDB("trust-base", dbm.LevelDBBackend, rootDir)),
	)
	source := lclient.NewProvider(chainID, client)

	// TODO: Make this more secure, e.g. make it interactive in the console?
	_, err := trust.LatestFullCommit(chainID, 1, 1<<63-1)
	if err != nil {
		fc, err := source.LatestFullCommit(chainID, 1, 1)
		if err != nil {
			return nil, err
		}
		err = trust.SaveFullCommit(fc)
		if err != nil {
			return nil, err
		}
	}

	cert, err := lite.NewInquiringCertifier(chainID, trust, source)
	if err != nil {
		return nil, err
	}

	return cert, nil
}
