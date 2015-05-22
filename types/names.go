package types

type NameRegEntry struct {
	Name    []byte // registered name for the entry
	Owner   []byte // address that created the entry
	Data    []byte // binary encoded byte array
	Expires uint   // block at which this entry expires
}
