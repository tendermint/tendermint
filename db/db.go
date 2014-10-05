package db

type DB interface {
	Get([]byte) []byte
	Set([]byte, []byte)
}
