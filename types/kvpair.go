package types

// KVPairInt is a helper method to build KV pair with an integer value.
func KVPairInt(key string, val int64) *KVPair {
	return &KVPair{
		Key:       key,
		ValueInt:  val,
		ValueType: KVPair_INT,
	}
}

// KVPairString is a helper method to build KV pair with a string value.
func KVPairString(key, val string) *KVPair {
	return &KVPair{
		Key:         key,
		ValueString: val,
		ValueType:   KVPair_STRING,
	}
}
