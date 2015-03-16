package vm

import ()

type Receipt struct {
	Index   uint
	Address []byte
	Topics  [][]byte
	Data    []byte
}

func (self *Receipt) String() string {
	return fmt.Sprintf("[A=%x T=%x D=%x]", self.Address, self.Topics, self.Data)
}
