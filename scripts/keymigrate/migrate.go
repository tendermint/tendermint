// Package keymigrate translates all legacy formatted keys to their
// new components.
//
// The key migration operation as implemented provides a potential
// model for database migration operations. Crucially, the migration
// as implemented does not depend on any tendermint code.
package keymigrate

import (
	"bytes"
	"context"
	"encoding/binary"
	"encoding/hex"
	"fmt"
	"math/rand"
	"runtime"
	"strconv"
	"sync"

	"github.com/google/orderedcode"
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
		// tx index
		keyID("tx.height/"),
		keyID("tx.hash/"),
	} {
		if bytes.HasPrefix(key, prefix) {
			return true
		}
	}

	// this means it's a tx index...
	if bytes.Count(key, []byte("/")) >= 3 {
		return true
	}

	return keyIsHash(key)
}

func keyIsHash(key keyID) bool {
	return len(key) == 32 && !bytes.Contains(key, []byte("/"))
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
		parts := bytes.Split(key[2:], []byte(":"))
		if len(parts) != 2 {
			return nil, fmt.Errorf("block parts key has %d rather than 2 components",
				len(parts))
		}
		valOne, err := strconv.Atoi(string(parts[0]))
		if err != nil {
			return nil, err
		}

		valTwo, err := strconv.Atoi(string(parts[1]))
		if err != nil {
			return nil, err
		}

		return orderedcode.Append(nil, int64(1), int64(valOne), int64(valTwo))
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
		return convertEvidence(key, 9)
	case bytes.HasPrefix(key, []byte{0x01}): // pending evidence
		return convertEvidence(key, 10)
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
	case bytes.HasPrefix(key, keyID("tx.height")):
		parts := bytes.Split(key, []byte("/"))
		if len(parts) != 4 {
			return nil, fmt.Errorf("key has %d parts rather than 4", len(parts))
		}
		parts = parts[1:] // drop prefix

		elems := make([]interface{}, 0, len(parts)+1)
		elems = append(elems, "tx.height")

		for idx, pt := range parts {
			val, err := strconv.Atoi(string(pt))
			if err != nil {
				return nil, err
			}
			if idx == 0 {
				elems = append(elems, fmt.Sprintf("%d", val))
			} else {
				elems = append(elems, int64(val))
			}
		}

		return orderedcode.Append(nil, elems...)
	case bytes.Count(key, []byte("/")) >= 3: // tx indexer
		parts := bytes.Split(key, []byte("/"))

		elems := make([]interface{}, 0, 4)
		if len(parts) == 4 {
			elems = append(elems, string(parts[0]), string(parts[1]))

			val, err := strconv.Atoi(string(parts[2]))
			if err != nil {
				return nil, err
			}
			elems = append(elems, int64(val))

			val2, err := strconv.Atoi(string(parts[3]))
			if err != nil {
				return nil, err
			}
			elems = append(elems, int64(val2))
		} else {
			elems = append(elems, string(parts[0]))
			parts = parts[1:]

			val, err := strconv.Atoi(string(parts[len(parts)-1]))
			if err != nil {
				return nil, err
			}

			val2, err := strconv.Atoi(string(parts[len(parts)-2]))
			if err != nil {
				return nil, err
			}

			appKey := bytes.Join(parts[:len(parts)-3], []byte("/"))
			elems = append(elems, string(appKey), int64(val), int64(val2))
		}
		return orderedcode.Append(nil, elems...)
	case keyIsHash(key):
		return orderedcode.Append(nil, "tx.hash", string(key))
	default:
		return nil, fmt.Errorf("key %q is in the wrong format", string(key))
	}
}

func convertEvidence(key keyID, newPrefix int64) ([]byte, error) {
	parts := bytes.Split(key[1:], []byte("/"))
	if len(parts) != 2 {
		return nil, fmt.Errorf("evidence key is malformed with %d parts not 2",
			len(parts))
	}

	hb, err := hex.DecodeString(string(parts[0]))
	if err != nil {
		return nil, err
	}

	evidenceHash, err := hex.DecodeString(string(parts[1]))
	if err != nil {
		return nil, err
	}

	return orderedcode.Append(nil, newPrefix, binary.BigEndian.Uint64(hb), string(evidenceHash))
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
	if rand.Intn(100)%10 == 0 { // nolint:gosec
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
//
// The context allows for a safe termination of the operation
// (e.g connected to a singal handler,) to abort the operation
// in-between migration operations.
func Migrate(ctx context.Context, db dbm.DB) error {
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

				if ctx.Err() != nil {
					return
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
		return fmt.Errorf("encountered errors during migration: %v", errStrs)
	}

	return nil
}
