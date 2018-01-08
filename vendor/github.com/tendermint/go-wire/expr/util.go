package expr

import (
	"strings"
)

func Compile(exprStr string) ([]byte, error) {
	exprObj, err := ParseReader("", strings.NewReader(exprStr))
	if err != nil {
		return nil, err
	}
	bz, err := exprObj.(Byteful).Bytes()
	if err != nil {
		return nil, err
	}
	return bz, err
}
