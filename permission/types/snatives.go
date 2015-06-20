package types

const (
	// first 32 bits of BasePermission are for chain, second 32 are for snative
	FirstSNativePerm PermFlag = 1 << 32
)

// we need to reset iota with no const block
const (
	// each snative has an associated permission flag
	HasBasePerm PermFlag = FirstSNativePerm << iota
	SetBasePerm
	UnsetBasePerm
	SetGlobalPerm
	ClearBasePerm
	HasRole
	AddRole
	RmRole

	// XXX: must be adjusted if snative's added/removed
	NumSNativePermissions uint     = 8
	TopSNativePermission  PermFlag = FirstSNativePerm << (NumSNativePermissions - 1)
	AllSNativePermissions PermFlag = (TopSNativePermission | (TopSNativePermission - 1)) &^ (FirstSNativePerm - 1)
)
