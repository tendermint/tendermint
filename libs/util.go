package libs

// Int64Ptr returns a pointer of passed int64 value
func Int64Ptr(n int64) *int64 {
	return &n
}

// BoolPtr returns a pointer of passed bool value
func BoolPtr(b bool) *bool {
	return &b
}

// BoolValue returns a boolean value of a passed pointer
func BoolValue(b *bool) bool {
	if b == nil {
		return false
	}
	return *b
}
