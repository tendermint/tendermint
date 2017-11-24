package types

type BaseApplication struct {
}

func NewBaseApplication() *BaseApplication {
	return &BaseApplication{}
}

func (BaseApplication) Info(req RequestInfo) ResponseInfo {
	return ResponseInfo{}
}

func (BaseApplication) SetOption(key string, value string) (log string) {
	return ""
}

func (BaseApplication) DeliverTx(tx []byte) ResponseDeliverTx {
	return ResponseDeliverTx{Code: CodeType_OK}
}

func (BaseApplication) CheckTx(tx []byte) ResponseCheckTx {
	return ResponseCheckTx{Code: CodeType_OK}
}

func (BaseApplication) Commit() ResponseCommit {
	return ResponseCommit{Code: CodeType_OK, Data: []byte("nil")}
}

func (BaseApplication) Query(req RequestQuery) ResponseQuery {
	return ResponseQuery{Code: CodeType_OK}
}

func (BaseApplication) InitChain(req RequestInitChain) {
}

func (BaseApplication) BeginBlock(req RequestBeginBlock) {
}

func (BaseApplication) EndBlock(height uint64) ResponseEndBlock {
	return ResponseEndBlock{}
}
