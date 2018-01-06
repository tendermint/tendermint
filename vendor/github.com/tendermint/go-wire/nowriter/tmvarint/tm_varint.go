package tmvarint

type TMVarint interface {
	EncodeUvarint(i uint) []byte
	EncodeVarint(i int) []byte
}
