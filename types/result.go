package types

import (
	"fmt"
)

type Result struct {
	Code CodeType
	Data []byte
	Log  string // Can be non-deterministic
}

func NewResult(code CodeType, data []byte, log string) Result {
	return Result{
		Code: code,
		Data: data,
		Log:  log,
	}
}

func (res Result) IsOK() bool {
	return res.Code == CodeType_OK
}

func (res Result) Error() string {
	return fmt.Sprintf("TMSP error code:%v, data:%X, log:%v", res.Code, res.Data, res.Log)
}

//----------------------------------------

func NewResultOK(data []byte, log string) Result {
	return Result{
		Code: CodeType_OK,
		Data: data,
		Log:  log,
	}
}
