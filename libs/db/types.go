package db

// DBs are goroutine safe.
type DB interface {

	// Get returns nil iff key doesn't exist.
	// A nil key is interpreted as an empty byteslice.
	// CONTRACT: key, value readonly []byte
	Get([]byte) []byte

	// Has checks if a key exists.
	// A nil key is interpreted as an empty byteslice.
	// CONTRACT: key, value readonly []byte
	Has(key []byte) bool

	// Set sets the key.
	// A nil key is interpreted as an empty byteslice.
	// CONTRACT: key, value readonly []byte
	Set([]byte, []byte)
	SetSync([]byte, []byte)

	// Delete deletes the key.
	// A nil key is interpreted as an empty byteslice.
	// CONTRACT: key readonly []byte
	Delete([]byte)
	DeleteSync([]byte)

	// Iterate over a domain of keys in ascending order. End is exclusive.
	// Start must be less than end, or the Iterator is invalid.
	// A nil start is interpreted as an empty byteslice.
	// If end is nil, iterates up to the last item (inclusive).
	// CONTRACT: No writes may happen within a domain while an iterator exists over it.
	// CONTRACT: start, end readonly []byte
	Iterator(start, end []byte) Iterator

	// Iterate over a domain of keys in descending order. End is exclusive.
	// Start must be less than end, or the Iterator is invalid.
	// If start is nil, iterates up to the first/least item (inclusive).
	// If end is nil, iterates from the last/greatest item (inclusive).
	// CONTRACT: No writes may happen within a domain while an iterator exists over it.
	// CONTRACT: start, end readonly []byte
	ReverseIterator(start, end []byte) Iterator

	// Closes the connection.
	Close()

	// Creates a batch for atomic updates.
	NewBatch() Batch

	// For debugging
	Print()

	// Stats returns a map of property values for all keys and the size of the cache.
	Stats() map[string]string
}

//----------------------------------------
// Batch

type Batch interface {
	SetDeleter
	Write()
	WriteSync()
}

type SetDeleter interface {
	Set(key, value []byte) // CONTRACT: key, value readonly []byte
	Delete(key []byte)     // CONTRACT: key readonly []byte
}

//----------------------------------------
// Iterator

/*
	Usage:

	var itr Iterator = ...
	defer itr.Close()

	for ; itr.Valid(); itr.Next() {
		k, v := itr.Key(); itr.Value()
		// ...
	}
*/
type Iterator interface {

	// The start & end (exclusive) limits to iterate over.
	// If end < start, then the Iterator goes in reverse order.
	//
	// A domain of ([]byte{12, 13}, []byte{12, 14}) will iterate
	// over anything with the prefix []byte{12, 13}.
	//
	// The smallest key is the empty byte array []byte{} - see BeginningKey().
	// The largest key is the nil byte array []byte(nil) - see EndingKey().
	// CONTRACT: start, end readonly []byte
	Domain() (start []byte, end []byte)

	// Valid returns whether the current position is valid.
	// Once invalid, an Iterator is forever invalid.
	Valid() bool

	// Next moves the iterator to the next sequential key in the database, as
	// defined by order of iteration.
	//
	// If Valid returns false, this method will panic.
	Next()

	// Key returns the key of the cursor.
	// If Valid returns false, this method will panic.
	// CONTRACT: key readonly []byte
	Key() (key []byte)

	// Value returns the value of the cursor.
	// If Valid returns false, this method will panic.
	// CONTRACT: value readonly []byte
	Value() (value []byte)

	// Close releases the Iterator.
	Close()
}

// For testing convenience.
func bz(s string) []byte {
	return []byte(s)
}

// We defensively turn nil keys or values into []byte{} for
// most operations.
func nonNilBytes(bz []byte) []byte {
	if bz == nil {
		return []byte{}
	}
	return bz
}
