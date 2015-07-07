package types

const (
	// first 32 bits of BasePermission are for chain, second 32 are for snative
	FirstSNativePermFlag PermFlag = 1 << 32
)

// we need to reset iota with no const block
const (
	// each snative has an associated permission flag
	HasBasePerm PermFlag = FirstSNativePermFlag << iota
	SetBasePerm
	UnsetBasePerm
	SetGlobalPerm
	ClearBasePerm
	HasRole
	AddRole
	RmRole
	NumSNativePermissions uint = 8 // NOTE adjust this too

	TopSNativePermFlag  PermFlag = FirstSNativePermFlag << (NumSNativePermissions - 1)
	AllSNativePermFlags PermFlag = (TopSNativePermFlag | (TopSNativePermFlag - 1)) &^ (FirstSNativePermFlag - 1)
)
