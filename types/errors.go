package types

var (
	OK = NewResultOK(nil, "")

	ErrInternalError     = NewError(CodeType_InternalError, "Internal error")
	ErrEncodingError     = NewError(CodeType_EncodingError, "Encoding error")
	ErrBadNonce          = NewError(CodeType_BadNonce, "Error bad nonce")
	ErrUnauthorized      = NewError(CodeType_Unauthorized, "Unauthorized")
	ErrInsufficientFunds = NewError(CodeType_InsufficientFunds, "Insufficient funds")
	ErrUnknownRequest    = NewError(CodeType_UnknownRequest, "Unknown request")

	ErrBaseDuplicateAddress     = NewError(CodeType_BaseDuplicateAddress, "Error duplicate address")
	ErrBaseEncodingError        = NewError(CodeType_BaseEncodingError, "Error encoding error")
	ErrBaseInsufficientFees     = NewError(CodeType_BaseInsufficientFees, "Error insufficient fees")
	ErrBaseInsufficientFunds    = NewError(CodeType_BaseInsufficientFunds, "Error insufficient funds")
	ErrBaseInsufficientGasPrice = NewError(CodeType_BaseInsufficientGasPrice, "Error insufficient gas price")
	ErrBaseInvalidAddress       = NewError(CodeType_BaseInvalidAddress, "Error invalid address")
	ErrBaseInvalidAmount        = NewError(CodeType_BaseInvalidAmount, "Error invalid amount")
	ErrBaseInvalidPubKey        = NewError(CodeType_BaseInvalidPubKey, "Error invalid pubkey")
	ErrBaseInvalidSequence      = NewError(CodeType_BaseInvalidSequence, "Error invalid sequence")
	ErrBaseInvalidSignature     = NewError(CodeType_BaseInvalidSignature, "Error invalid signature")
	ErrBaseUnknownPubKey        = NewError(CodeType_BaseUnknownPubKey, "Error unknown pubkey")
)
