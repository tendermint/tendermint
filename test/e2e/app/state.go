//nolint: gosec
package app

import (
	"crypto/sha256"
	"encoding/binary"
	"encoding/json"
	"errors"
	"fmt"
	"io/ioutil"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"sync"
)

const stateFileName = "app_state_%06d.json"

// State is the application state.
type State struct {
	sync.RWMutex
	Height uint64
	Values map[string]string
	Hash   []byte

	// private fields aren't marshaled to disk.
	// app saves current and historical states for rollback functionality
	stateFileTemplate string

	persistInterval uint64
	initialHeight   uint64
	path            string
}

// NewState creates a new state.
func NewState(dir string, persistInterval uint64) (*State, error) {
	state := &State{
		Values:            make(map[string]string),
		stateFileTemplate: filepath.Join(dir, stateFileName),
		persistInterval:   persistInterval,
		path:              dir,
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

func (s *State) currentFile() string {
	return fmt.Sprintf(s.stateFileTemplate, s.Height)
}

func (s *State) previousFile() (string, error) {
	files, err := ioutil.ReadDir(s.path)
	if err != nil {
		return "", errors.New("failed to read the tendermint app folder")
	}

	sizes := len(files)
	if sizes == 0 {
		return "", errors.New("empty data folder")
	}

	foundCurrentFile := false
	for i := sizes - 1; i >= 0; i-- {
		fmt.Println(files[i].Name())
		if files[i].IsDir() || strings.Contains(files[i].Name(), "new") {
			continue
		}

		if filepath.Join(s.path, files[i].Name()) == s.currentFile() {
			foundCurrentFile = true
			continue
		}

		if foundCurrentFile {
			return filepath.Join(s.path, files[i].Name()), nil
		}
	}

	return "", errors.New("no previous state file found")
}

// load loads state from disk. It does not take out a lock, since it is called
// during construction.
func (s *State) load() error {
	files, err := ioutil.ReadDir(s.path)
	if err != nil {
		return fmt.Errorf("failed to read the tendermint state folder: %w", err)
	}

	sizes := len(files)
	if sizes == 0 {
		return fmt.Errorf("empty data folder: %w", os.ErrNotExist)
	}

	// the folder stores the app_state with different versions, and the snapshots (in the snapshots subfolder)
	// when the app loads the files from it, the files contain is sorted in ascending order.
	retry := 1
	for i := sizes - 1; i >= 0; i-- {
		if files[i].IsDir() || strings.Contains(files[i].Name(), "new") {
			continue
		}

		bz, err := os.ReadFile(filepath.Join(s.path, files[i].Name()))
		if err != nil {
			// if the current state doesn't exist then we try recover from the previous state
			if errors.Is(err, os.ErrNotExist) && retry > 0 {
				retry--
				continue
			} else {
				return fmt.Errorf("failed to read state from %q: %w", files[i].Name(), err)
			}
		}

		err = json.Unmarshal(bz, s)
		if err != nil {
			return fmt.Errorf("invalid state data in %q: %w", files[i].Name(), err)
		}
		return nil
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
	newFile := fmt.Sprintf("%v.new", s.currentFile())
	err = os.WriteFile(newFile, bz, 0644)
	if err != nil {
		return fmt.Errorf("failed to write state to %q: %w", s.currentFile(), err)
	}
	// Finally, we take the new state and replace the current state.
	return os.Rename(newFile, s.currentFile())
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

// Commit commits the current state.
func (s *State) Commit() (uint64, []byte, error) {
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
	if s.persistInterval > 0 && s.Height%s.persistInterval == 0 {
		err := s.save()
		if err != nil {
			return 0, nil, err
		}
	}
	return s.Height, s.Hash, nil
}

func (s *State) Rollback() error {
	prevFile, err := s.previousFile()
	if err != nil {
		return fmt.Errorf("failed to find previous state file: %w", err)
	}
	bz, err := os.ReadFile(prevFile)
	if err != nil {
		return fmt.Errorf("failed to read state from %q: %w", prevFile, err)
	}
	err = json.Unmarshal(bz, s)
	if err != nil {
		return fmt.Errorf("invalid state data in %q: %w", prevFile, err)
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
