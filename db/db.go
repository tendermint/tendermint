package db

type Db interface {
	Get([]byte) []byte
	Set([]byte, []byte)
}
