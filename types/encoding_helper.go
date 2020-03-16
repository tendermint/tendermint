package types

// TODO: depreacte (remove)
// cdcEncode returns nil if the input is nil, otherwise returns
func cdcEncode(item interface{}) []byte {
	if item != nil && !isTypedNil(item) && !isEmpty(item) {
		return cdc.MustMarshalBinaryBare(item)
	}
	return nil
}
