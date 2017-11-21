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

func (BaseApplication) DeliverTx(tx []byte) Result {
	return NewResultOK(nil, "")
}

func (BaseApplication) CheckTx(tx []byte) Result {
	return NewResultOK(nil, "")
}

func (BaseApplication) Commit() Result {
	return NewResultOK([]byte("nil"), "")
}

func (BaseApplication) Query(req RequestQuery) ResponseQuery {
	return ResponseQuery{}
}

func (BaseApplication) InitChain(req RequestInitChain) {
}

func (BaseApplication) BeginBlock(req RequestBeginBlock) {
}

func (BaseApplication) EndBlock(height uint64) ResponseEndBlock {
	return ResponseEndBlock{}
}
