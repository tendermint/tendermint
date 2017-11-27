package types

var (
	code2string = map[CodeType]string{
		CodeType_InternalError:     "Internal error",
		CodeType_EncodingError:     "Encoding error",
		CodeType_BadNonce:          "Error bad nonce",
		CodeType_Unauthorized:      "Unauthorized",
		CodeType_InsufficientFunds: "Insufficient funds",
		CodeType_UnknownRequest:    "Unknown request",

		CodeType_BaseDuplicateAddress:     "Error (base) duplicate address",
		CodeType_BaseEncodingError:        "Error (base) encoding error",
		CodeType_BaseInsufficientFees:     "Error (base) insufficient fees",
		CodeType_BaseInsufficientFunds:    "Error (base) insufficient funds",
		CodeType_BaseInsufficientGasPrice: "Error (base) insufficient gas price",
		CodeType_BaseInvalidInput:         "Error (base) invalid input",
		CodeType_BaseInvalidOutput:        "Error (base) invalid output",
		CodeType_BaseInvalidPubKey:        "Error (base) invalid pubkey",
		CodeType_BaseInvalidSequence:      "Error (base) invalid sequence",
		CodeType_BaseInvalidSignature:     "Error (base) invalid signature",
		CodeType_BaseUnknownAddress:       "Error (base) unknown address",
		CodeType_BaseUnknownPlugin:        "Error (base) unknown plugin",
		CodeType_BaseUnknownPubKey:        "Error (base) unknown pubkey",
	}
)

func (c CodeType) IsOK() bool { return c == CodeType_OK }

// HumanCode transforms code into a more humane format, such as "Internal error" instead of 0.
func HumanCode(code CodeType) string {
	s, ok := code2string[code]
	if !ok {
		return "Unknown code"
	}
	return s
}
