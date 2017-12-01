package types

type BaseApplication struct {
}

func NewBaseApplication() *BaseApplication {
	return &BaseApplication{}
}

func (BaseApplication) Info(req RequestInfo) ResponseInfo {
	return ResponseInfo{}
}

func (BaseApplication) SetOption(req RequestSetOption) ResponseSetOption {
	return ResponseSetOption{}
}

func (BaseApplication) DeliverTx(tx []byte) ResponseDeliverTx {
	return ResponseDeliverTx{Code: CodeTypeOK}
}

func (BaseApplication) CheckTx(tx []byte) ResponseCheckTx {
	return ResponseCheckTx{Code: CodeTypeOK}
}

func (BaseApplication) Commit() ResponseCommit {
	return ResponseCommit{Code: CodeTypeOK}
}

func (BaseApplication) Query(req RequestQuery) ResponseQuery {
	return ResponseQuery{Code: CodeTypeOK}
}

func (BaseApplication) InitChain(req RequestInitChain) ResponseInitChain {
	return ResponseInitChain{}
}

func (BaseApplication) BeginBlock(req RequestBeginBlock) ResponseBeginBlock {
	return ResponseBeginBlock{}
}

func (BaseApplication) EndBlock(req RequestEndBlock) ResponseEndBlock {
	return ResponseEndBlock{}
}
