package types

import (
	"crypto/sha256"
	"fmt"
	"io/ioutil"
	"os"
	"path/filepath"
	"strconv"
)

const (
	ManifestVersion int32 = 0
	ChunkPayloadMaxBytes int = 4 * 1024 * 1024 // 4M before compression
	snapshotDir string = "snapshot"
	finalizedDir string = "current"
	restorationDir string = "restoration"
	manifestFileName string = "MANIFEST"
)

// AppStateChunk completeness enums
// We don't use enum because amino doesn't support encode enum
const (
	Complete uint8 = 0
	InComplete_First uint8 = 1
	InComplete_Mid uint8 = 2
	InComplete_Last uint8 = 3
)

type SHA256Sum [sha256.Size]byte // check sum of chunk

type Manifest struct {
	Version int32 // snapshot Version

	Height         int64       // the height of this snapshot
	StateHashes    []SHA256Sum // hashes of state chunks
	AppStateHashes []SHA256Sum // hashes of app state chunks
	BlockHashes    []SHA256Sum // Block hashes
	NumKeys        []int64     // num of keys for each substore, this sacrifices clear boundary between cosmos and tendermint, saying tendermint knows applicaction db might organized by substores. But reduce network/disk pressure that each chunk must has a field indicates what's his store
}

func NewManifest(
	height int64,
	stateHashes []SHA256Sum,
	appStateHashes []SHA256Sum,
	blockHashes []SHA256Sum,
	numKeys []int64) Manifest {
	return Manifest{
		ManifestVersion,
		height,
		stateHashes,
		appStateHashes,
		blockHashes,
		numKeys,
	}
}

type SnapshotChunk interface{}

type StateChunk struct {
	Statepart []byte
}

/**
 * Completeness:
 * 	0 - all nodes in Nodes field are complete in this chunk
 *  1 - last node in Nodes field is not complete, and its the first node of following incomplete chunks sequence
 *  2 - there is only one incomplete part of node in Nodes which follows previous chunk
 *  3 - there is only one incomplete part of node in Nodes and its the last part of previous chunks
 */
type AppStateChunk struct {
	StartIdx     int64    // compare (startIdx and number of complete nodes) against (Manifest.NumKeys) we know each node's substore
	Completeness uint8    // flag of completeness of this chunk, not enum because of go-amino doesn't support enum encoding
	Nodes        [][]byte // iavl tree serialized node, one big node (i.e. active orders and orderbook) might be split into different chunks (complete is flag to indicate that), ordering is ensured by list on manifest
}

// TODO: should we make block chunk to be blockpart?
// current max block size is 100M while normal chunk should be less than 4M
// but as blocks usually not big
// so we don't split block just like blockchaind reactor (in fast sync)
type BlockChunk struct {
	Block      []byte // amino encoded block
	SeenCommit []byte // amino encoded Commit - we need this because Block only keep LastSeenCommit, for commit of this block, we need load it in same way it was saved
}

// read snapshot manifest and chunks from disk
type SnapshotReader struct {
	Height int64
	DbDir  string
}

func (reader *SnapshotReader) Load(hash SHA256Sum) ([]byte, error) {
	return reader.loadImpl(hash, finalizedDir)
}

func (reader *SnapshotReader) LoadFromRestoration(hash SHA256Sum) ([]byte, error) {
	return reader.loadImpl(hash, restorationDir)
}

func (reader *SnapshotReader) loadImpl(hash SHA256Sum, category string) ([]byte, error) {
	toRead := filepath.Join(reader.DbDir, snapshotDir, strconv.FormatInt(reader.Height, 10), category, fmt.Sprintf("%x", hash))
	return ioutil.ReadFile(toRead)
}

func (reader *SnapshotReader) LoadManifest(height int64) (int64, []byte, error) {
	var lookupHeight int64
	if height == 0 {
		lookupHeight = reader.Height
	}
	if lookupHeight == 0 {
		return 0, nil, fmt.Errorf("requested wrong height: %d, reader height: %d", height, reader.Height)
	} else {
		toRead := filepath.Join(reader.DbDir, snapshotDir, strconv.FormatInt(lookupHeight, 10), finalizedDir, manifestFileName)
		manifest, err := ioutil.ReadFile(toRead)
		return lookupHeight, manifest, err
	}
}

func (reader *SnapshotReader) IsFinalized() bool {
	toCheck := filepath.Join(reader.DbDir, snapshotDir, strconv.FormatInt(reader.Height, 10), finalizedDir)
	_, err := os.Stat(toCheck)
	return err == nil
}

func (reader *SnapshotReader) LatestSnapshotHeight() int64 {
	var latestHeight int64

	toTraverse := filepath.Join(reader.DbDir, snapshotDir)
	if files, err := ioutil.ReadDir(toTraverse); err == nil {
		for _, f := range files {
			if f.IsDir() {
				if height, err := strconv.ParseInt(f.Name(), 10, 64); err == nil && height > latestHeight {
					if _, err := os.Stat(filepath.Join(toTraverse, f.Name(), finalizedDir)); !os.IsNotExist(err) {
						latestHeight = height
					}
				}
			}
		}
	}

	reader.Height = latestHeight
	return latestHeight
}

// write snapshot manifest and chunks to disk
type SnapshotWriter struct {
	Height int64
	DbDir  string
}

func (writer *SnapshotWriter) Write(hash SHA256Sum, chunk []byte) error {
	path := filepath.Join(writer.DbDir, snapshotDir, strconv.FormatInt(writer.Height, 10), restorationDir)
	if err := os.MkdirAll(path, os.ModePerm); err != nil {
		return err
	}
	toWrite := filepath.Join(path, fmt.Sprintf("%x", hash))
	return ioutil.WriteFile(toWrite, chunk, 0644)
}

func (writer *SnapshotWriter) WriteManifest(manifest []byte) error {
	path := filepath.Join(writer.DbDir, snapshotDir, strconv.FormatInt(writer.Height, 10), restorationDir)
	if err := os.MkdirAll(path, os.ModePerm); err != nil {
		return err
	}
	toWrite := filepath.Join(path, manifestFileName)
	return ioutil.WriteFile(toWrite, manifest, 0644)
}

func (writer *SnapshotWriter) Finalize() error {
	baseDir := filepath.Join(writer.DbDir, snapshotDir, strconv.FormatInt(writer.Height, 10))
	return os.Rename(filepath.Join(baseDir, restorationDir), filepath.Join(baseDir, finalizedDir))
}

func (writer *SnapshotWriter) Delete() error {
	return os.RemoveAll(filepath.Join(writer.DbDir, snapshotDir, strconv.FormatInt(writer.Height, 10)))
}
