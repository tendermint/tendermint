package db

type prefixDBBatch struct {
	prefix []byte
	source Batch
}

var _ Batch = (*prefixDBBatch)(nil)

func newPrefixBatch(prefix []byte, source Batch) prefixDBBatch {
	return prefixDBBatch{
		prefix: prefix,
		source: source,
	}
}

// Set implements Batch.
func (pb prefixDBBatch) Set(key, value []byte) error {
	pkey := append(cp(pb.prefix), key...)
	return pb.source.Set(pkey, value)
}

// Delete implements Batch.
func (pb prefixDBBatch) Delete(key []byte) error {
	pkey := append(cp(pb.prefix), key...)
	return pb.source.Delete(pkey)
}

// Write implements Batch.
func (pb prefixDBBatch) Write() error {
	return pb.source.Write()
}

// WriteSync implements Batch.
func (pb prefixDBBatch) WriteSync() error {
	return pb.source.WriteSync()
}

// Close implements Batch.
func (pb prefixDBBatch) Close() error {
	return pb.source.Close()
}
