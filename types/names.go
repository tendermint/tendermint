package types

import (
	"regexp"
)

var (
	MinNameRegistrationPeriod int = 5

	// NOTE: base costs and validity checks are here so clients
	// can use them without importing state

	// cost for storing a name for a block is
	// CostPerBlock*CostPerByte*(len(data) + 32)
	NameCostPerByte  int64 = 1
	NameCostPerBlock int64 = 1

	MaxNameLength = 32
	MaxDataLength = 1 << 16

	// Name should be alphanum, underscore, slash
	// Data should be anything permitted in JSON
	regexpAlphaNum = regexp.MustCompile("^[a-zA-Z0-9._/]*$")
	regexpJSON     = regexp.MustCompile(`^[a-zA-Z0-9_/ \-"':,\n\t.{}()\[\]]*$`)
)

// filter strings
func validateNameRegEntryName(name string) bool {
	return regexpAlphaNum.Match([]byte(name))
}

func validateNameRegEntryData(data string) bool {
	return regexpJSON.Match([]byte(data))
}

// base cost is "effective" number of bytes
func BaseEntryCost(name, data string) int64 {
	return int64(len(data) + 32)
}

type NameRegEntry struct {
	Name    string `json:"name"`    // registered name for the entry
	Owner   []byte `json:"owner"`   // address that created the entry
	Data    string `json:"data"`    // data to store under this name
	Expires int    `json:"expires"` // block at which this entry expires
}

func (entry *NameRegEntry) Copy() *NameRegEntry {
	entryCopy := *entry
	return &entryCopy
}
