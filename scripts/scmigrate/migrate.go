// Package scmigrate implements a migration for SeenCommit data
// between 0.34 and 0.35
//
// The Migrate implementation is idempotent and finds all seen commit
// records and deletes all *except* the record corresponding to the
// highest height.
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

func makeKeyFromPrefix(ids ...int64) []byte {
	vals := make([]interface{}, len(ids))
	for idx := range ids {
		vals[idx] = ids[idx]
	}

	key, err := orderedcode.Append(nil, vals...)
	if err != nil {
		panic(err)
	}
	return key
}

func makeToMigrate(val []byte) (*types.Commit, error) {
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
		commit, err := makeToMigrate(iter.Value())
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

func renameRecord(db dbm.DB, keep toMigrate) error {
	wantKey := makeKeyFromPrefix(prefixSeenCommit)
	if bytes.Equal(keep.key, wantKey) {
		return nil // we already did this conversion
	}

	// This record's key has already been converted to the "new" format, we just
	// now need to trim off the tail.
	val, err := db.Get(keep.key)
	if err != nil {
		return err
	}

	batch := db.NewBatch()
	if err := batch.Delete(keep.key); err != nil {
		return err
	}
	if err := batch.Set(wantKey, val); err != nil {
		return err
	}
	werr := batch.Write()
	cerr := batch.Close()
	if werr != nil {
		return werr
	}
	return cerr
}

func deleteRecords(db dbm.DB, scData []toMigrate) error {
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
	} else if len(scData) == 0 {
		return nil // nothing to do
	}

	// Sort commits in decreasing order of height.
	sortMigrations(scData)

	// Keep and rename the newest seen commit, delete the rest.
	// In TM < v0.35 we kept a last-seen commit for each height; in v0.35 we
	// retain only the latest.
	keep, remove := scData[0], scData[1:]

	if err := renameRecord(db, keep); err != nil {
		return fmt.Errorf("renaming seen commit record: %w", err)
	}

	if len(remove) == 0 {
		return nil
	}

	// Remove any older seen commits. Prior to v0.35, we kept these records for
	// all heights, but v0.35 keeps only the latest.
	if err := deleteRecords(db, remove); err != nil {
		return fmt.Errorf("writing data: %w", err)
	}

	return nil
}
