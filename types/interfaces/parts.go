package interfaces

type PartSet interface{}

type PartSetHeader interface {
	IsZero() bool // checks if the total == 0 or len of the hash == 0
	Equals(other PartSetHeader) bool
	// ValidateBasic performs basic validation.
	ValidateBasic() error
}
