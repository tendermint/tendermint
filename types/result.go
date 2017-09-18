package types

import (
	"fmt"

	"github.com/tendermint/go-wire/data"
)

// Result is a common result object for ABCI calls.
// CONTRACT: a zero Result is OK.
type Result struct {
	Code CodeType   `json:"code"`
	Data data.Bytes `json:"data"`
	Log  string     `json:"log"` // Can be non-deterministic
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

func (res Result) IsErr() bool {
	return res.Code != CodeType_OK
}

func (res Result) IsSameCode(compare Result) bool {
	return res.Code == compare.Code
}

func (res Result) Error() string {
	return fmt.Sprintf("ABCI{code:%v, data:%X, log:%v}", res.Code, res.Data, res.Log)
}

func (res Result) String() string {
	return fmt.Sprintf("ABCI{code:%v, data:%X, log:%v}", res.Code, res.Data, res.Log)
}

func (res Result) PrependLog(log string) Result {
	return Result{
		Code: res.Code,
		Data: res.Data,
		Log:  log + ";" + res.Log,
	}
}

func (res Result) AppendLog(log string) Result {
	return Result{
		Code: res.Code,
		Data: res.Data,
		Log:  res.Log + ";" + log,
	}
}

func (res Result) SetLog(log string) Result {
	return Result{
		Code: res.Code,
		Data: res.Data,
		Log:  log,
	}
}

func (res Result) SetData(data []byte) Result {
	return Result{
		Code: res.Code,
		Data: data,
		Log:  res.Log,
	}
}

//----------------------------------------

// NOTE: if data == nil and log == "", same as zero Result.
func NewResultOK(data []byte, log string) Result {
	return Result{
		Code: CodeType_OK,
		Data: data,
		Log:  log,
	}
}

func NewError(code CodeType, log string) Result {
	return Result{
		Code: code,
		Log:  log,
	}
}

//----------------------------------------
// Convenience methods for turning the
// pb type into one using data.Bytes

// Convert ResponseCheckTx to standard Result
func (r *ResponseCheckTx) Result() Result {
	return Result{
		Code: r.Code,
		Data: r.Data,
		Log:  r.Log,
	}
}

// Convert ResponseDeliverTx to standard Result
func (r *ResponseDeliverTx) Result() Result {
	return Result{
		Code: r.Code,
		Data: r.Data,
		Log:  r.Log,
	}
}

type ResultQuery struct {
	Code   CodeType   `json:"code"`
	Index  int64      `json:"index"`
	Key    data.Bytes `json:"key"`
	Value  data.Bytes `json:"value"`
	Proof  data.Bytes `json:"proof"`
	Height uint64     `json:"height"`
	Log    string     `json:"log"`
}

func (r *ResponseQuery) Result() *ResultQuery {
	return &ResultQuery{
		Code:   r.Code,
		Index:  r.Index,
		Key:    r.Key,
		Value:  r.Value,
		Proof:  r.Proof,
		Height: r.Height,
		Log:    r.Log,
	}
}
