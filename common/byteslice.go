package common

func Fingerprint(bytez []byte) []byte {
	fingerprint := make([]byte, 6)
	copy(fingerprint, bytez)
	return fingerprint
}
