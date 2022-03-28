package scmigrate

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"sort"

	"github.com/gogo/protobuf/proto"
	"github.com/google/orderedcode"
	dbm "github.com/tendermint/tm-db"

	tmproto "github.com/tendermint/tendermint/proto/tendermint/types"
	"github.com/tendermint/tendermint/types"
)

type toMigrate struct {
	key    []byte
	commit *types.Commit
}

const prefixSeenCommit = int64(3)

func makeKeyFromPrefix(ids ...interface{}) []byte {
	key, err := orderedcode.Append(nil, ids...)
	if err != nil {
		panic(err)
	}
	return key
}

func makeToMigratge(val []byte) (*types.Commit, error) {
	if len(val) == 0 {
		return nil, errors.New("empty value")
	}

	var pbc = new(tmproto.Commit)

	if err := proto.Unmarshal(val, pbc); err != nil {
		return nil, fmt.Errorf("error reading block seen commit: %w", err)
	}

	commit, err := types.CommitFromProto(pbc)
	if commit == nil {
		// theoretically we should error for all errors, but
		// there's no reason to keep junk data in the
		// database, and it makes testing easier.
		if err != nil {
			return nil, fmt.Errorf("error from proto commit: %w", err)
		}
		return nil, fmt.Errorf("missing commit")
	}

	return commit, nil
}

func sortMigrations(scData []toMigrate) {
	// put this in it's own function just to make it testable
	sort.SliceStable(scData, func(i, j int) bool {
		return scData[i].commit.Height > scData[j].commit.Height
	})
}

func getMigrationsToDelete(in []toMigrate) []toMigrate { return in[1:] }

func getAllSeenCommits(ctx context.Context, db dbm.DB) ([]toMigrate, error) {
	scKeyPrefix := makeKeyFromPrefix(prefixSeenCommit)
	iter, err := db.Iterator(
		scKeyPrefix,
		makeKeyFromPrefix(prefixSeenCommit+1),
	)
	if err != nil {
		return nil, err
	}

	scData := []toMigrate{}
	for ; iter.Valid(); iter.Next() {
		if err := ctx.Err(); err != nil {
			return nil, err
		}

		k := iter.Key()
		nk := make([]byte, len(k))
		copy(nk, k)

		if !bytes.HasPrefix(nk, scKeyPrefix) {
			break
		}
		commit, err := makeToMigratge(iter.Value())
		if err != nil {
			return nil, err
		}

		scData = append(scData, toMigrate{
			key:    nk,
			commit: commit,
		})
	}
	if err := iter.Error(); err != nil {
		return nil, err
	}
	if err := iter.Close(); err != nil {
		return nil, err
	}
	return scData, nil
}

func migrateRecords(ctx context.Context, db dbm.DB, scData []toMigrate) error {
	// delete all the remaining stale values in a single batch
	batch := db.NewBatch()

	for _, mg := range scData {
		if err := batch.Delete(mg.key); err != nil {
			return err
		}
	}

	if err := batch.WriteSync(); err != nil {
		return err
	}

	if err := batch.Close(); err != nil {
		return err
	}
	return nil
}

func Migrate(ctx context.Context, db dbm.DB) error {
	scData, err := getAllSeenCommits(ctx, db)
	if err != nil {
		return fmt.Errorf("sourcing tasks to migrate: %w", err)
	}

	// sort earliest->latest commits.
	sortMigrations(scData)

	// trim the one we want to save:
	scData = getMigrationsToDelete(scData)

	if len(scData) <= 1 {
		return nil
	}

	// write the migration (remove )
	if err := migrateRecords(ctx, db, scData); err != nil {
		return fmt.Errorf("writing data: %w", err)
	}

	return nil
}
