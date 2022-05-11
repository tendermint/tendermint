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

	"github.com/creachadair/taskgroup"
	"github.com/google/orderedcode"
	dbm "github.com/tendermint/tm-db"
)

type (
	keyID       []byte
	migrateFunc func(keyID) (keyID, error)
)

func getAllLegacyKeys(db dbm.DB) ([]keyID, error) {
	var out []keyID

	iter, err := db.Iterator(nil, nil)
	if err != nil {
		return nil, err
	}

	for ; iter.Valid(); iter.Next() {
		k := iter.Key()

		// make sure it's a key with a legacy format, and skip
		// all other keys, to make it safe to resume the migration.
		if !checkKeyType(k).isLegacy() {
			continue
		}

		// Make an explicit copy, since not all tm-db backends do.
		out = append(out, []byte(string(k)))
	}

	if err = iter.Error(); err != nil {
		return nil, err
	}

	if err = iter.Close(); err != nil {
		return nil, err
	}

	return out, nil
}

// keyType is an enumeration for the structural type of a key.
type keyType int

func (t keyType) isLegacy() bool { return t != nonLegacyKey }

const (
	nonLegacyKey keyType = iota // non-legacy key (presumed already converted)
	consensusParamsKey
	abciResponsesKey
	validatorsKey
	stateStoreKey        // state storage record
	blockMetaKey         // H:
	blockPartKey         // P:
	commitKey            // C:
	seenCommitKey        // SC:
	blockHashKey         // BH:
	lightSizeKey         // size
	lightBlockKey        // lb/
	evidenceCommittedKey // \x00
	evidencePendingKey   // \x01
	txHeightKey          // tx.height/... (special case)
	abciEventKey         // name/value/height/index
	txHashKey            // 32-byte transaction hash (unprefixed)
)

var prefixes = []struct {
	prefix []byte
	ktype  keyType
	check  func(keyID) bool
}{
	{[]byte("consensusParamsKey:"), consensusParamsKey, nil},
	{[]byte("abciResponsesKey:"), abciResponsesKey, nil},
	{[]byte("validatorsKey:"), validatorsKey, nil},
	{[]byte("stateKey"), stateStoreKey, nil},
	{[]byte("H:"), blockMetaKey, nil},
	{[]byte("P:"), blockPartKey, nil},
	{[]byte("C:"), commitKey, nil},
	{[]byte("SC:"), seenCommitKey, nil},
	{[]byte("BH:"), blockHashKey, nil},
	{[]byte("size"), lightSizeKey, nil},
	{[]byte("lb/"), lightBlockKey, nil},
	{[]byte("\x00"), evidenceCommittedKey, checkEvidenceKey},
	{[]byte("\x01"), evidencePendingKey, checkEvidenceKey},
}

// checkKeyType classifies a candidate key based on its structure.
func checkKeyType(key keyID) keyType {
	for _, p := range prefixes {
		if bytes.HasPrefix(key, p.prefix) {
			if p.check == nil || p.check(key) {
				return p.ktype
			}
		}
	}

	// A legacy event key has the form:
	//
	//    <name> / <value> / <height> / <index>
	//
	// Transaction hashes are stored as a raw binary hash with no prefix.
	//
	// Because a hash can contain any byte, it is possible (though unlikely)
	// that a hash could have the correct form for an event key, in which case
	// we would translate it incorrectly.  To reduce the likelihood of an
	// incorrect interpretation, we parse candidate event keys and check for
	// some structural properties before making a decision.
	//
	// Note, though, that nothing prevents event names or values from containing
	// additional "/" separators, so the parse has to be forgiving.
	parts := bytes.Split(key, []byte("/"))
	if len(parts) >= 4 {
		// Special case for tx.height.
		if len(parts) == 4 && bytes.Equal(parts[0], []byte("tx.height")) {
			return txHeightKey
		}

		// The name cannot be empty, but we don't know where the name ends and
		// the value begins, so insist that there be something.
		var n int
		for _, part := range parts[:len(parts)-2] {
			n += len(part)
		}
		// Check whether the last two fields could be .../height/index.
		if n > 0 && isDecimal(parts[len(parts)-1]) && isDecimal(parts[len(parts)-2]) {
			return abciEventKey
		}
	}

	// If we get here, it's not an event key. Treat it as a hash if it is the
	// right length. Note that it IS possible this could collide with the
	// translation of some other key (though not a hash, since encoded hashes
	// will be longer). The chance of that is small, but there is nothing we can
	// do to detect it.
	if len(key) == 32 {
		return txHashKey
	}
	return nonLegacyKey
}

// isDecimal reports whether buf is a non-empty sequence of Unicode decimal
// digits.
func isDecimal(buf []byte) bool {
	for _, c := range buf {
		if c < '0' || c > '9' {
			return false
		}
	}
	return len(buf) != 0
}

func migrateKey(key keyID) (keyID, error) {
	switch checkKeyType(key) {
	case blockMetaKey:
		val, err := strconv.Atoi(string(key[2:]))
		if err != nil {
			return nil, err
		}

		return orderedcode.Append(nil, int64(0), int64(val))
	case blockPartKey:
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
	case commitKey:
		val, err := strconv.Atoi(string(key[2:]))
		if err != nil {
			return nil, err
		}

		return orderedcode.Append(nil, int64(2), int64(val))
	case seenCommitKey:
		val, err := strconv.Atoi(string(key[3:]))
		if err != nil {
			return nil, err
		}

		return orderedcode.Append(nil, int64(3), int64(val))
	case blockHashKey:
		hash := string(key[3:])
		if len(hash)%2 == 1 {
			hash = "0" + hash
		}
		val, err := hex.DecodeString(hash)
		if err != nil {
			return nil, err
		}

		return orderedcode.Append(nil, int64(4), string(val))
	case validatorsKey:
		val, err := strconv.Atoi(string(key[14:]))
		if err != nil {
			return nil, err
		}

		return orderedcode.Append(nil, int64(5), int64(val))
	case consensusParamsKey:
		val, err := strconv.Atoi(string(key[19:]))
		if err != nil {
			return nil, err
		}

		return orderedcode.Append(nil, int64(6), int64(val))
	case abciResponsesKey:
		val, err := strconv.Atoi(string(key[17:]))
		if err != nil {
			return nil, err
		}

		return orderedcode.Append(nil, int64(7), int64(val))
	case stateStoreKey:
		return orderedcode.Append(nil, int64(8))
	case evidenceCommittedKey:
		return convertEvidence(key, 9)
	case evidencePendingKey:
		return convertEvidence(key, 10)
	case lightBlockKey:
		if len(key) < 24 {
			return nil, fmt.Errorf("light block evidence %q in invalid format", string(key))
		}

		val, err := strconv.Atoi(string(key[len(key)-20:]))
		if err != nil {
			return nil, err
		}

		return orderedcode.Append(nil, int64(11), int64(val))
	case lightSizeKey:
		return orderedcode.Append(nil, int64(12))
	case txHeightKey:
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
	case abciEventKey:
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
	case txHashKey:
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

// checkEvidenceKey reports whether a candidate key with one of the legacy
// evidence prefixes has the correct structure for a legacy evidence key.
//
// This check is needed because transaction hashes are stored without a prefix,
// so checking the one-byte prefix alone is not enough to distinguish them.
// Legacy evidence keys are suffixed with a string of the format:
//
//     "%0.16X/%X"
//
// where the first element is the height and the second is the hash.  Thus, we
// check
func checkEvidenceKey(key keyID) bool {
	parts := bytes.SplitN(key[1:], []byte("/"), 2)
	if len(parts) != 2 || len(parts[0]) != 16 || !isHex(parts[0]) || !isHex(parts[1]) {
		return false
	}
	return true
}

func isHex(data []byte) bool {
	for _, b := range data {
		if ('0' <= b && b <= '9') || ('a' <= b && b <= 'f') || ('A' <= b && b <= 'F') {
			continue
		}
		return false
	}
	return len(data) != 0
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

	var errs []string
	g, start := taskgroup.New(func(err error) error {
		errs = append(errs, err.Error())
		return err
	}).Limit(runtime.NumCPU())

	for _, key := range keys {
		key := key
		start(func() error {
			if err := ctx.Err(); err != nil {
				return err
			}
			return replaceKey(db, key, migrateKey)
		})
	}
	if g.Wait() != nil {
		return fmt.Errorf("encountered errors during migration: %q", errs)
	}
	return nil
}
