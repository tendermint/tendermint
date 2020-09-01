package snapshots

import (
	"bufio"
	"compress/zlib"
	"errors"
	"fmt"
	"io"
	"math"
	"sort"
	"strings"

	"github.com/cosmos/iavl"
	snapshottypes "github.com/tendermint/tendermint/abci/example/iavlstore/snapshots/types"
	"github.com/tendermint/tendermint/libs/protoio"
)

const (
	snapshotChunkSize   = uint64(10e6)
	snapshotBufferSize  = int(snapshotChunkSize)
	snapshotMaxItemSize = int(64e6) // SDK has no key/value size limit, so we set an arbitrary limit
)

// IAVLSnapshotter is a wrapper around an IAVL store that does snapshotting.
// It implements the Snapshotter interface.
type IAVLSnapshotter struct {
	*iavl.MutableTree
}

// Snapshot implements Snapshotter.
func (tree *IAVLSnapshotter) Snapshot(height uint64, format uint32) (<-chan io.ReadCloser, error) {
	if format != snapshottypes.CurrentFormat {
		return nil, fmt.Errorf("%w %v", snapshottypes.ErrUnknownFormat, format)
	}
	if height == 0 {
		return nil, errors.New("cannot snapshot height 0")
	}
	if height > uint64(tree.Version()) {
		return nil, fmt.Errorf("cannot snapshot future height %v", height)
	}

	// Collect stores to snapshot (only IAVL stores are supported)
	type namedStore struct {
		*iavl.MutableTree
		name string
	}
	stores := []namedStore{{
		MutableTree: tree.MutableTree,
		name:        "iavlstore",
	}}
	sort.Slice(stores, func(i, j int) bool {
		return strings.Compare(stores[i].name, stores[j].name) == -1
	})

	// Spawn goroutine to generate snapshot chunks and pass their io.ReadClosers through a channel
	ch := make(chan io.ReadCloser)
	go func() {
		// Set up a stream pipeline to serialize snapshot nodes:
		// ExportNode -> delimited Protobuf -> zlib -> buffer -> chunkWriter -> chan io.ReadCloser
		chunkWriter := NewChunkWriter(ch, snapshotChunkSize)
		defer chunkWriter.Close()
		bufWriter := bufio.NewWriterSize(chunkWriter, snapshotBufferSize)
		defer func() {
			if err := bufWriter.Flush(); err != nil {
				chunkWriter.CloseWithError(err)
			}
		}()
		zWriter, err := zlib.NewWriterLevel(bufWriter, 7)
		if err != nil {
			chunkWriter.CloseWithError(fmt.Errorf("zlib error: %w", err))
			return
		}
		defer func() {
			if err := zWriter.Close(); err != nil {
				chunkWriter.CloseWithError(err)
			}
		}()
		protoWriter := protoio.NewDelimitedWriter(zWriter)
		defer func() {
			if err := protoWriter.Close(); err != nil {
				chunkWriter.CloseWithError(err)
			}
		}()

		// Export each IAVL store. Stores are serialized as a stream of SnapshotItem Protobuf
		// messages. The first item contains a SnapshotStore with store metadata (i.e. name),
		// and the following messages contain a SnapshotNode (i.e. an ExportNode). Store changes
		// are demarcated by new SnapshotStore items.
		for _, store := range stores {
			tree, err := store.GetImmutable(int64(height))
			if err != nil {
				chunkWriter.CloseWithError(err)
				return
			}
			exporter := tree.Export()
			defer exporter.Close()
			_, err = protoWriter.WriteMsg(&snapshottypes.SnapshotItem{
				Item: &snapshottypes.SnapshotItem_Store{
					Store: &snapshottypes.SnapshotStoreItem{
						Name: store.name,
					},
				},
			})
			if err != nil {
				chunkWriter.CloseWithError(err)
				return
			}

			for {
				node, err := exporter.Next()
				if err == iavl.ExportDone {
					break
				} else if err != nil {
					chunkWriter.CloseWithError(err)
					return
				}
				_, err = protoWriter.WriteMsg(&snapshottypes.SnapshotItem{
					Item: &snapshottypes.SnapshotItem_IAVL{
						IAVL: &snapshottypes.SnapshotIAVLItem{
							Key:     node.Key,
							Value:   node.Value,
							Height:  int32(node.Height),
							Version: node.Version,
						},
					},
				})
				if err != nil {
					chunkWriter.CloseWithError(err)
					return
				}
			}
			exporter.Close()
		}
	}()

	return ch, nil
}

// Restore implements snapshottypes.Snapshotter.
func (tree *IAVLSnapshotter) Restore(
	height uint64, format uint32, chunks <-chan io.ReadCloser, ready chan<- struct{},
) error {
	if format != snapshottypes.CurrentFormat {
		return fmt.Errorf("%w %v", snapshottypes.ErrUnknownFormat, format)
	}
	if height == 0 {
		return fmt.Errorf("%w: cannot restore snapshot at height 0", snapshottypes.ErrInvalidMetadata)
	}
	if height > math.MaxInt64 {
		return fmt.Errorf("%w: snapshot height %v cannot exceed %v", snapshottypes.ErrInvalidMetadata,
			height, math.MaxInt64)
	}

	// Signal readiness. Must be done before the readers below are set up, since the zlib
	// reader reads from the stream on initialization, potentially causing deadlocks.
	if ready != nil {
		close(ready)
	}

	// Set up a restore stream pipeline
	// chan io.ReadCloser -> chunkReader -> zlib -> delimited Protobuf -> ExportNode
	chunkReader := NewChunkReader(chunks)
	defer chunkReader.Close()
	zReader, err := zlib.NewReader(chunkReader)
	if err != nil {
		return fmt.Errorf("zlib error: %w", err)
	}
	defer zReader.Close()
	protoReader := protoio.NewDelimitedReader(zReader, snapshotMaxItemSize)
	defer protoReader.Close()

	// Import nodes into stores. The first item is expected to be a SnapshotItem containing
	// a SnapshotStoreItem, telling us which store to import into. The following items will contain
	// SnapshotNodeItem (i.e. ExportNode) until we reach the next SnapshotStoreItem or EOF.
	var importer *iavl.Importer
	for {
		item := &snapshottypes.SnapshotItem{}
		err := protoReader.ReadMsg(item)
		if err == io.EOF {
			break
		} else if err != nil {
			return fmt.Errorf("invalid protobuf message: %w", err)
		}

		switch item := item.Item.(type) {
		case *snapshottypes.SnapshotItem_Store:
			if importer != nil {
				err = importer.Commit()
				if err != nil {
					return fmt.Errorf("IAVL commit failed: %w", err)
				}
				importer.Close()
			}
			if item.Store.Name != "iavlstore" {
				return fmt.Errorf("cannot import into non-IAVL store %q", item.Store.Name)
			}
			importer, err = tree.Import(int64(height))
			if err != nil {
				return fmt.Errorf("import failed: %w", err)
			}
			defer importer.Close()

		case *snapshottypes.SnapshotItem_IAVL:
			if importer == nil {
				return fmt.Errorf("received IAVL node item before store item")
			}
			if item.IAVL.Height > math.MaxInt8 {
				return fmt.Errorf("node height %v cannot exceed %v", item.IAVL.Height, math.MaxInt8)
			}
			node := &iavl.ExportNode{
				Key:     item.IAVL.Key,
				Value:   item.IAVL.Value,
				Height:  int8(item.IAVL.Height),
				Version: item.IAVL.Version,
			}
			// Protobuf does not differentiate between []byte{} as nil, but fortunately IAVL does
			// not allow nil keys nor nil values for leaf nodes, so we can always set them to empty.
			if node.Key == nil {
				node.Key = []byte{}
			}
			if node.Height == 0 && node.Value == nil {
				node.Value = []byte{}
			}
			err := importer.Add(node)
			if err != nil {
				return fmt.Errorf("IAVL node import failed: %w", err)
			}

		default:
			return fmt.Errorf("unknown snapshot item %T", item)
		}
	}

	if importer != nil {
		err := importer.Commit()
		if err != nil {
			return fmt.Errorf("IAVL commit failed: %w", err)
		}
		importer.Close()
	}

	_, err = tree.Load()
	return err
}
