package types

type NameRegEntry struct {
	Name    string `json:"name"`    // registered name for the entry
	Owner   []byte `json:"owner"`   // address that created the entry
	Data    string `json:"data"`    // binary encoded byte array
	Expires uint64 `json:"expires"` // block at which this entry expires
}

func (entry *NameRegEntry) Copy() *NameRegEntry {
	entryCopy := *entry
	return &entryCopy
}
