package peer

/*  Filter

    A Filter could be a bloom filter for lossy filtering, or could be a lossless filter.
    Either way, it's used to keep track of what a peer knows of.
*/
type Filter interface {
    Binary
    Add(Msg)
    Has(Msg)    bool

    // Loads a new filter.
    // Convenience factory method
    Load(ByteSlice) Filter
}
