//nolint: gosec
package app

import (
	"crypto/sha256"
	"encoding/binary"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"sync"
)

const stateFileName = "app_state.json"
const prevStateFileName = "prev_app_state.json"

// State is the application state.
type State struct {
	sync.RWMutex
	Height uint64
	Values map[string]string
	Hash   []byte

	// private fields aren't marshaled to disk.
	currentFile string
	// app saves current and previous state for rollback functionality
	previousFile    string
	persistInterval uint64
	initialHeight   uint64
}

// NewState creates a new state.
func NewState(dir string, persistInterval uint64) (*State, error) {
	state := &State{
		Values:          make(map[string]string),
		currentFile:     filepath.Join(dir, stateFileName),
		previousFile:    filepath.Join(dir, prevStateFileName),
		persistInterval: persistInterval,
	}
	state.Hash = hashItems(state.Values, state.Height)
	err := state.load()
	switch {
	case errors.Is(err, os.ErrNotExist):
	case err != nil:
		return nil, err
	}
	return state, nil
}

// load loads state from disk. It does not take out a lock, since it is called
// during construction.
func (s *State) load() error {
	bz, err := os.ReadFile(s.currentFile)
	if err != nil {
		// if the current state doesn't exist then we try recover from the previous state
		if errors.Is(err, os.ErrNotExist) {
			bz, err = os.ReadFile(s.previousFile)
			if err != nil {
				return fmt.Errorf("failed to read both current and previous state (%q): %w",
					s.previousFile, err)
			}
		} else {
			return fmt.Errorf("failed to read state from %q: %w", s.currentFile, err)
		}
	}
	err = json.Unmarshal(bz, s)
	if err != nil {
		return fmt.Errorf("invalid state data in %q: %w", s.currentFile, err)
	}
	return nil
}

// save saves the state to disk. It does not take out a lock since it is called
// internally by Commit which does lock.
func (s *State) save() error {
	bz, err := json.Marshal(s)
	if err != nil {
		return fmt.Errorf("failed to marshal state: %w", err)
	}
	// We write the state to a separate file and move it to the destination, to
	// make it atomic.
	newFile := fmt.Sprintf("%v.new", s.currentFile)
	err = os.WriteFile(newFile, bz, 0644)
	if err != nil {
		return fmt.Errorf("failed to write state to %q: %w", s.currentFile, err)
	}
	// We take the current state and move it to the previous state, replacing it
	if _, err := os.Stat(s.currentFile); err == nil {
		if err := os.Rename(s.currentFile, s.previousFile); err != nil {
			return fmt.Errorf("failed to replace previous state: %w", err)
		}
	}
	// Finally, we take the new state and replace the current state.
	return os.Rename(newFile, s.currentFile)
}

// Export exports key/value pairs as JSON, used for state sync snapshots.
func (s *State) Export() ([]byte, error) {
	s.RLock()
	defer s.RUnlock()
	return json.Marshal(s.Values)
}

// Import imports key/value pairs from JSON bytes, used for InitChain.AppStateBytes and
// state sync snapshots. It also saves the state once imported.
func (s *State) Import(height uint64, jsonBytes []byte) error {
	s.Lock()
	defer s.Unlock()
	values := map[string]string{}
	err := json.Unmarshal(jsonBytes, &values)
	if err != nil {
		return fmt.Errorf("failed to decode imported JSON data: %w", err)
	}
	s.Height = height
	s.Values = values
	s.Hash = hashItems(values, height)
	return s.save()
}

// Get fetches a value. A missing value is returned as an empty string.
func (s *State) Get(key string) string {
	s.RLock()
	defer s.RUnlock()
	return s.Values[key]
}

// Set sets a value. Setting an empty value is equivalent to deleting it.
func (s *State) Set(key, value string) {
	s.Lock()
	defer s.Unlock()
	if value == "" {
		delete(s.Values, key)
	} else {
		s.Values[key] = value
	}
}

// Finalize is called after applying a block, updating the height and returning the new app_hash
func (s *State) Finalize() []byte {
	s.Lock()
	defer s.Unlock()
	switch {
	case s.Height > 0:
		s.Height++
	case s.initialHeight > 0:
		s.Height = s.initialHeight
	default:
		s.Height = 1
	}
	s.Hash = hashItems(s.Values, s.Height)
	return s.Hash
}

// Commit commits the current state.
func (s *State) Commit() (uint64, error) {
	s.Lock()
	defer s.Unlock()
	if s.persistInterval > 0 && s.Height%s.persistInterval == 0 {
		err := s.save()
		if err != nil {
			return 0, err
		}
	}
	return s.Height, nil
}

func (s *State) Rollback() error {
	bz, err := os.ReadFile(s.previousFile)
	if err != nil {
		return fmt.Errorf("failed to read state from %q: %w", s.previousFile, err)
	}
	err = json.Unmarshal(bz, s)
	if err != nil {
		return fmt.Errorf("invalid state data in %q: %w", s.previousFile, err)
	}
	return nil
}

// hashItems hashes a set of key/value items.
func hashItems(items map[string]string, height uint64) []byte {
	keys := make([]string, 0, len(items))
	for key := range items {
		keys = append(keys, key)
	}
	sort.Strings(keys)

	hasher := sha256.New()
	var b [8]byte
	binary.BigEndian.PutUint64(b[:], height)
	_, _ = hasher.Write(b[:])
	for _, key := range keys {
		_, _ = hasher.Write([]byte(key))
		_, _ = hasher.Write([]byte{0})
		_, _ = hasher.Write([]byte(items[key]))
		_, _ = hasher.Write([]byte{0})
	}
	return hasher.Sum(nil)
}
