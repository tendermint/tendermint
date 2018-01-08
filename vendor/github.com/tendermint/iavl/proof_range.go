package iavl

import (
	"bytes"
	"fmt"

	"github.com/pkg/errors"
	"github.com/tendermint/go-wire/data"
)

// KeyInRangeProof is an interface which covers both first-in-range and last-in-range proofs.
type KeyInRangeProof interface {
	Verify(startKey, endKey, key, value, root []byte) error
}

// KeyFirstInRangeProof is a proof that a given key is the first in a given range.
type KeyFirstInRangeProof struct {
	KeyExistsProof `json:"key_proof"`

	Left  *pathWithNode `json:"left"`
	Right *pathWithNode `json:"right"`
}

// String returns a string representation of the proof.
func (proof *KeyFirstInRangeProof) String() string {
	return fmt.Sprintf("%#v", proof)
}

// Verify that the first in range proof is valid.
func (proof *KeyFirstInRangeProof) Verify(startKey, endKey, key, value []byte, root []byte) error {
	if key != nil {
		inputsOutOfRange := bytes.Compare(key, startKey) == -1 || bytes.Compare(key, endKey) == 1
		if inputsOutOfRange {
			return ErrInvalidInputs
		}
	}
	if proof.Left == nil && proof.Right == nil && proof.PathToKey == nil {
		return errors.WithStack(ErrInvalidProof)
	}
	if err := verifyPaths(proof.Left, proof.Right, startKey, endKey, root); err != nil {
		return err
	}
	if proof.PathToKey == nil {
		// If we don't have an existing key, we effectively have a proof of absence.
		return verifyKeyAbsence(proof.Left, proof.Right)
	}

	if err := proof.KeyExistsProof.Verify(key, value, root); err != nil {
		return errors.Wrap(err, "failed to verify key exists proof")
	}
	// If the key returned is equal to our start key, and we've verified
	// that it exists, there's nothing else to check.
	if bytes.Equal(key, startKey) {
		return nil
	}
	// If the key returned is the smallest in the tree, then it must be
	// the smallest in the given range too.
	if proof.PathToKey.isLeftmost() {
		return nil
	}
	// The start key is in between the left path and the key returned,
	// and the paths are adjacent. Therefore there is nothing between
	// the key returned and the start key.
	if proof.Left != nil && proof.Left.Path.isLeftAdjacentTo(proof.PathToKey) {
		return nil
	}
	return errors.WithStack(ErrInvalidProof)
}

///////////////////////////////////////////////////////////////////////////////

// KeyLastInRangeProof is a proof that a given key is the last in a given range.
type KeyLastInRangeProof struct {
	KeyExistsProof `json:"key_proof"`

	Left  *pathWithNode `json:"left"`
	Right *pathWithNode `json:"right"`
}

// String returns a string representation of the proof.
func (proof *KeyLastInRangeProof) String() string {
	// TODO(cloudhead): Needs work.
	return fmt.Sprintf("%#v", proof)
}

// Verify that the last in range proof is valid.
func (proof *KeyLastInRangeProof) Verify(startKey, endKey, key, value []byte, root []byte) error {
	if key != nil && (bytes.Compare(key, startKey) == -1 || bytes.Compare(key, endKey) == 1) {
		return ErrInvalidInputs
	}
	if proof.Left == nil && proof.Right == nil && proof.PathToKey == nil {
		return errors.WithStack(ErrInvalidProof)
	}
	if err := verifyPaths(proof.Left, proof.Right, startKey, endKey, root); err != nil {
		return err
	}
	if proof.PathToKey == nil {
		// If we don't have an existing key, we effectively have a proof of absence.
		return verifyKeyAbsence(proof.Left, proof.Right)
	}

	if err := proof.KeyExistsProof.Verify(key, value, root); err != nil {
		return err
	}
	if bytes.Equal(key, endKey) {
		return nil
	}
	if proof.PathToKey.isRightmost() {
		return nil
	}
	if proof.Right != nil &&
		proof.PathToKey.isLeftAdjacentTo(proof.Right.Path) {
		return nil
	}

	return errors.WithStack(ErrInvalidProof)
}

///////////////////////////////////////////////////////////////////////////////

// KeyRangeProof is proof that a range of keys does or does not exist.
type KeyRangeProof struct {
	RootHash   data.Bytes   `json:"root_hash"`
	Version    uint64       `json:"version"`
	PathToKeys []*PathToKey `json:"paths"`

	Left  *pathWithNode `json:"left"`
	Right *pathWithNode `json:"right"`
}

// Verify that a range proof is valid.
//
// This method expects the same parameters passed to query the range.
func (proof *KeyRangeProof) Verify(
	startKey, endKey []byte, limit int, keys, values [][]byte, root []byte,
) error {
	if len(proof.PathToKeys) != len(keys) || len(values) != len(keys) {
		return ErrInvalidInputs
	}
	if limit > 0 && len(keys) > limit {
		return ErrInvalidInputs
	}

	// If startKey > endKey, reverse the keys and values, since our proofs are
	// always in ascending order.
	ascending := bytes.Compare(startKey, endKey) == -1
	if !ascending {
		startKey, endKey, keys, values = reverseKeys(startKey, endKey, keys, values)
	}

	// If the range is empty, we just have to check the left and right paths.
	if len(keys) == 0 {
		if err := verifyKeyAbsence(proof.Left, proof.Right); err != nil {
			return err
		}
		return verifyPaths(proof.Left, proof.Right, startKey, endKey, root)
	}

	// If we hit the limit, one of the two ends doesn't have to match the
	// limits of the query, so we adjust the range to match the limit we found.
	if limit > 0 && len(keys) == limit {
		if ascending {
			endKey = keys[len(keys)-1]
		} else {
			startKey = keys[0]
		}
	}
	// Now we know Left < startKey <= endKey < Right.
	if err := verifyPaths(proof.Left, proof.Right, startKey, endKey, root); err != nil {
		return err
	}

	if err := verifyNoMissingKeys(proof.paths()); err != nil {
		return errors.WithStack(err)
	}

	// If we've reached this point, it means our range isn't empty, and we have
	// a list of keys.
	for i, path := range proof.PathToKeys {
		leafNode := proofLeafNode{KeyBytes: keys[i], ValueBytes: values[i]}
		if err := path.verify(leafNode, root); err != nil {
			return errors.WithStack(err)
		}
	}

	// In the case of a descending range, if the left proof is nil and the
	// limit wasn't reached, we have to verify that we're not missing any
	// keys. Basically, if a key to the left is missing because we've
	// reached the limit, then it's fine. But if the key count is smaller
	// than the limit, we need a left proof to make sure no keys are
	// missing.
	if proof.Left == nil &&
		!bytes.Equal(startKey, keys[0]) &&
		!proof.PathToKeys[0].isLeftmost() {
		return errors.WithStack(ErrInvalidProof)
	}

	if proof.Right == nil &&
		!bytes.Equal(endKey, keys[len(keys)-1]) &&
		!proof.PathToKeys[len(proof.PathToKeys)-1].isRightmost() {
		return errors.WithStack(ErrInvalidProof)
	}
	return nil
}

func (proof *KeyRangeProof) String() string {
	// TODO(cloudhead): Needs work.
	return fmt.Sprintf("%#v", proof)
}

// Returns a list of all paths, in order, with the proof's Left and Right
// paths preprended and appended respectively, if they exist.
func (proof *KeyRangeProof) paths() []*PathToKey {
	paths := proof.PathToKeys[:]
	if proof.Left != nil {
		paths = append([]*PathToKey{proof.Left.Path}, paths...)
	}
	if proof.Right != nil {
		paths = append(paths, proof.Right.Path)
	}
	return paths
}

///////////////////////////////////////////////////////////////////////////////

func (t *Tree) getRangeWithProof(keyStart, keyEnd []byte, limit int) (
	keys, values [][]byte, rangeProof *KeyRangeProof, err error,
) {
	if t.root == nil {
		return nil, nil, nil, ErrNilRoot
	}
	t.root.hashWithCount() // Ensure that all hashes are calculated.

	rangeProof = &KeyRangeProof{RootHash: t.root.hash}
	rangeStart, rangeEnd := keyStart, keyEnd
	ascending := bytes.Compare(keyStart, keyEnd) == -1
	if !ascending {
		rangeStart, rangeEnd = rangeEnd, rangeStart
	}

	limited := t.IterateRangeInclusive(rangeStart, rangeEnd, ascending, func(k, v []byte) bool {
		keys = append(keys, k)
		values = append(values, v)
		return len(keys) == limit
	})

	// Construct the paths such that they are always in ascending order.
	rangeProof.PathToKeys = make([]*PathToKey, len(keys))
	for i, k := range keys {
		path, _, _ := t.root.pathToKey(t, k)
		if ascending {
			rangeProof.PathToKeys[i] = path
		} else {
			rangeProof.PathToKeys[len(keys)-i-1] = path
		}
	}

	//
	// Figure out which of the left or right paths we need.
	//
	var needsLeft, needsRight bool

	if len(keys) == 0 {
		needsLeft, needsRight = true, true
	} else {
		first, last := 0, len(keys)-1
		if !ascending {
			first, last = last, first
		}

		needsLeft = !bytes.Equal(keys[first], rangeStart)
		needsRight = !bytes.Equal(keys[last], rangeEnd)

		// When limited, we can relax the right or left side, depending on
		// the direction of the range.
		if limited {
			if ascending {
				needsRight = false
			} else {
				needsLeft = false
			}
		}
	}

	// So far, we've created proofs of the keys which are within the provided range.
	// Next, we need to create a proof that we haven't omitted any keys to the left
	// or right of that range. This is relevant in two scenarios:
	//
	// 1. There are no keys in the range. In this case, include a proof of the key
	//    to the left and right of that empty range.
	// 2. The start or end key do not match the start and end of the keys returned.
	//    In this case, include proofs of the keys immediately outside of those returned.
	//
	if needsLeft {
		// Find index of first key to the left, and include proof if it isn't the
		// leftmost key.
		if idx, _ := t.Get(rangeStart); idx > 0 {
			lkey, lval := t.GetByIndex(idx - 1)
			path, _, _ := t.root.pathToKey(t, lkey)
			rangeProof.Left = &pathWithNode{
				Path: path,
				Node: proofLeafNode{KeyBytes: lkey, ValueBytes: lval},
			}
		}
	}

	// Proof that the last key is the last value before keyEnd, or that we're limited.
	// If len(keys) == limit, it doesn't matter that a key exists to the right of the
	// last key, since we aren't interested in it.
	if needsRight {
		// Find index of first key to the right, and include proof if it isn't the
		// rightmost key.
		if idx, _ := t.Get(rangeEnd); idx <= t.Size()-1 {
			rkey, rval := t.GetByIndex(idx)
			path, _, _ := t.root.pathToKey(t, rkey)
			rangeProof.Right = &pathWithNode{
				Path: path,
				Node: proofLeafNode{KeyBytes: rkey, ValueBytes: rval},
			}
		}
	}

	return keys, values, rangeProof, nil
}

func (t *Tree) getFirstInRangeWithProof(keyStart, keyEnd []byte) (
	key, value []byte, proof *KeyFirstInRangeProof, err error,
) {
	if t.root == nil {
		return nil, nil, nil, ErrNilRoot
	}
	t.root.hashWithCount() // Ensure that all hashes are calculated.
	proof = &KeyFirstInRangeProof{}
	proof.RootHash = t.root.hash

	// Get the first value in the range.
	t.IterateRangeInclusive(keyStart, keyEnd, true, func(k, v []byte) bool {
		key, value = k, v
		return true
	})

	if len(key) > 0 {
		proof.PathToKey, _, _ = t.root.pathToKey(t, key)
	}

	if !bytes.Equal(key, keyStart) {
		if idx, _ := t.Get(keyStart); idx-1 >= 0 && idx-1 <= t.Size()-1 {
			k, v := t.GetByIndex(idx - 1)
			path, node, _ := t.root.pathToKey(t, k)
			proof.Left = &pathWithNode{
				Path: path,
				Node: proofLeafNode{k, v, node.version},
			}
		}
	}

	if !bytes.Equal(key, keyEnd) {
		if idx, val := t.Get(keyEnd); idx <= t.Size()-1 && val == nil {
			k, v := t.GetByIndex(idx)
			path, node, _ := t.root.pathToKey(t, k)
			proof.Right = &pathWithNode{
				Path: path,
				Node: proofLeafNode{k, v, node.version},
			}
		}
	}

	return key, value, proof, nil
}

func (t *Tree) getLastInRangeWithProof(keyStart, keyEnd []byte) (
	key, value []byte, proof *KeyLastInRangeProof, err error,
) {
	if t.root == nil {
		return nil, nil, nil, ErrNilRoot
	}
	t.root.hashWithCount() // Ensure that all hashes are calculated.

	proof = &KeyLastInRangeProof{}
	proof.RootHash = t.root.hash

	// Get the last value in the range.
	t.IterateRangeInclusive(keyStart, keyEnd, false, func(k, v []byte) bool {
		key, value = k, v
		return true
	})

	if len(key) > 0 {
		proof.PathToKey, _, _ = t.root.pathToKey(t, key)
	}

	if !bytes.Equal(key, keyEnd) {
		if idx, _ := t.Get(keyEnd); idx <= t.Size()-1 {
			k, v := t.GetByIndex(idx)
			path, node, _ := t.root.pathToKey(t, k)
			proof.Right = &pathWithNode{
				Path: path,
				Node: proofLeafNode{k, v, node.version},
			}
		}
	}

	if !bytes.Equal(key, keyStart) {
		if idx, _ := t.Get(keyStart); idx-1 >= 0 && idx-1 <= t.Size()-1 {
			k, v := t.GetByIndex(idx - 1)
			path, node, _ := t.root.pathToKey(t, k)
			proof.Left = &pathWithNode{
				Path: path,
				Node: proofLeafNode{k, v, node.version},
			}
		}
	}

	return key, value, proof, nil
}

///////////////////////////////////////////////////////////////////////////////

// reverseKeys reverses the keys and values and swaps start and end key
// if startKey > endKey.
func reverseKeys(startKey, endKey []byte, keys, values [][]byte) (
	[]byte, []byte, [][]byte, [][]byte,
) {
	if bytes.Compare(startKey, endKey) == 1 {
		startKey, endKey = endKey, startKey

		ks := make([][]byte, len(keys))
		vs := make([][]byte, len(keys))
		for i, _ := range keys {
			ks[len(ks)-1-i] = keys[i]
			vs[len(vs)-1-i] = values[i]
		}
		keys, values = ks, vs
	}
	return startKey, endKey, keys, values
}
