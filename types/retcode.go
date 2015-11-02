package types

type RetCode int

// Reserved return codes
const (
	RetCodeOK               = RetCode(0)
	RetCodeInternalError    = RetCode(1)
	RetCodeUnauthorized     = RetCode(2)
	RetCodeInsufficientFees = RetCode(3)
	RetCodeUnknownRequest   = RetCode(4)
)
