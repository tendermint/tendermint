package common

import "crypto/rand"

func RandStr(length int) string {
	b := make([]byte, length)
	_, err := rand.Read(b)
	if err != nil {
		return ""
	}
	return string(b)
}
