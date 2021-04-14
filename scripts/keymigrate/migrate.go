// Package keymigrate translates all legacy formatted keys to their
// new components.
//
// The key migration operation as implemented provides a potential
// model for database migration operations. Crucially, the migration
// as implemented does not depend on any tendermint code.
package keymigrate

import (
	"bytes"
	"fmt"
	"math/rand"
	"runtime"
	"strconv"
	"sync"

	"github.com/google/orderedcode"
	"github.com/pkg/errors"
	dbm "github.com/tendermint/tm-db"
)

type (
	keyID       []byte
	migrateFunc func(keyID) (keyID, error)
)

func getAllLegacyKeys(db dbm.DB) ([]keyID, error) {
	out := []keyID{}

	iter, err := db.Iterator(nil, nil)
	if err != nil {
		return nil, err
	}

	for ; iter.Valid(); iter.Next() {
		k := iter.Key()

		// make sure it's a key with a legacy format, and skip
		// all other keys, to make it safe to resume the migration.
		if !keyIsLegacy(k) {
			continue
		}

		// there's inconsistency around tm-db's handling of
		// key copies.
		nk := make([]byte, len(k))
		copy(nk, k)
		out = append(out, nk)
	}

	if err = iter.Error(); err != nil {
		return nil, err
	}

	if err = iter.Close(); err != nil {
		return nil, err
	}

	return out, nil
}

func makeKeyChan(keys []keyID) <-chan keyID {
	out := make(chan keyID, len(keys))
	defer close(out)

	for _, key := range keys {
		out <- key
	}

	return out
}

func keyIsLegacy(key keyID) bool {
	for _, prefix := range []keyID{
		// core "store"
		keyID("consensusParamsKey:"),
		keyID("abciResponsesKey:"),
		keyID("validatorsKey:"),
		keyID("stateKey"),
		keyID("H:"),
		keyID("P:"),
		keyID("C:"),
		keyID("SC:"),
		keyID("BH:"),
		// light
		keyID("size"),
		keyID("lb/"),
		// evidence
		keyID([]byte{0x00}),
		keyID([]byte{0x01}),
	} {
		if bytes.HasPrefix(key, prefix) {
			return true
		}
	}

	return false
}

func migarateKey(key keyID) (keyID, error) {
	switch {
	case bytes.HasPrefix(key, keyID("H:")):
		val, err := strconv.Atoi(string(key[2:]))
		if err != nil {
			return nil, err
		}

		return orderedcode.Append(nil, int64(0), int64(val))
	case bytes.HasPrefix(key, keyID("P:")):
		val, err := strconv.Atoi(string(key[2:]))
		if err != nil {
			return nil, err
		}

		return orderedcode.Append(nil, int64(1), int64(val))
	case bytes.HasPrefix(key, keyID("C:")):
		val, err := strconv.Atoi(string(key[2:]))
		if err != nil {
			return nil, err
		}

		return orderedcode.Append(nil, int64(2), int64(val))
	case bytes.HasPrefix(key, keyID("SC:")):
		val, err := strconv.Atoi(string(key[3:]))
		if err != nil {
			return nil, err
		}

		return orderedcode.Append(nil, int64(3), int64(val))
	case bytes.HasPrefix(key, keyID("BH:")):
		val, err := strconv.Atoi(string(key[3:]))
		if err != nil {
			return nil, err
		}

		return orderedcode.Append(nil, int64(4), int64(val))
	case bytes.HasPrefix(key, keyID("validatorsKey:")):
		val, err := strconv.Atoi(string(key[14:]))
		if err != nil {
			return nil, err
		}

		return orderedcode.Append(nil, int64(5), int64(val))
	case bytes.HasPrefix(key, keyID("consensusParamsKey:")):
		val, err := strconv.Atoi(string(key[19:]))
		if err != nil {
			return nil, err
		}

		return orderedcode.Append(nil, int64(6), int64(val))
	case bytes.HasPrefix(key, keyID("abciResponsesKey:")):
		val, err := strconv.Atoi(string(key[17:]))
		if err != nil {
			return nil, err
		}

		return orderedcode.Append(nil, int64(7), int64(val))
	case bytes.HasPrefix(key, keyID("stateKey")):
		return orderedcode.Append(nil, int64(8))
	case bytes.HasPrefix(key, []byte{0x00}): // committed evidence
		return orderedcode.Append(nil, int64(9))
	case bytes.HasPrefix(key, []byte{0x01}): // pending evidence
		return orderedcode.Append(nil, int64(10))
	case bytes.HasPrefix(key, keyID("lb/")):
		if len(key) < 24 {
			return nil, fmt.Errorf("light block evidence %q in invalid format", string(key))
		}

		val, err := strconv.Atoi(string(key[len(key)-20:]))
		if err != nil {
			return nil, err
		}

		return orderedcode.Append(nil, int64(11), int64(val))
	case bytes.HasPrefix(key, keyID("size")):
		return orderedcode.Append(nil, int64(12))
	default:
		return nil, fmt.Errorf("key %q is in the wrong format", string(key))
	}
}

func replaceKey(db dbm.DB, key keyID, gooseFn migrateFunc) error {
	exists, err := db.Has(key)
	if err != nil {
		return err
	}
	if !exists {
		return nil
	}

	newKey, err := gooseFn(key)
	if err != nil {
		return err
	}

	val, err := db.Get(key)
	if err != nil {
		return err
	}

	batch := db.NewBatch()

	if err = batch.Set(newKey, val); err != nil {
		return err
	}
	if err = batch.Delete(key); err != nil {
		return err
	}

	// 10% of the time, force a write to disk, but mostly don't,
	// because it's faster.
	if rand.Intn(100)%10 == 0 {
		if err = batch.WriteSync(); err != nil {
			return err
		}
	} else {
		if err = batch.Write(); err != nil {
			return err
		}
	}

	if err = batch.Close(); err != nil {
		return err
	}

	return nil
}

// Migrate converts all legacy key formats to new key formats. The
// operation is idempotent, so it's safe to resume a failed
// operation. The operation is somewhat parallelized, relying on the
// concurrency safety of the underlying databases.
//
// Migrate has "continue on error" semantics and will iterate through
// all legacy keys attempt to migrate them, and will collect all
// errors and will return only at the end of the operation.
func Migrate(db dbm.DB) error {
	keys, err := getAllLegacyKeys(db)
	if err != nil {
		return err
	}

	numWorkers := runtime.NumCPU()
	wg := &sync.WaitGroup{}

	errs := make(chan error, numWorkers)

	keyCh := makeKeyChan(keys)

	// run migrations.
	for i := 0; i < numWorkers; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for key := range keyCh {
				err := replaceKey(db, key, migarateKey)
				if err != nil {
					errs <- err
				}
			}
		}()
	}

	// collect and process the errors.
	errStrs := []string{}
	signal := make(chan struct{})
	go func() {
		defer close(signal)
		for err := range errs {
			if err == nil {
				continue
			}
			errStrs = append(errStrs, err.Error())
		}
	}()

	// Wait for everything to be done.
	wg.Wait()
	close(errs)
	<-signal

	// check the error results
	if len(errs) != 0 {
		return errors.Errorf("encountered errors during migration: %v", errStrs)
	}

	return nil
}
