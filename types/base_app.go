package types

type BaseApplication struct {
}

func NewBaseApplication() *BaseApplication {
	return &BaseApplication{}
}

func (app *BaseApplication) Info(req RequestInfo) (resInfo ResponseInfo) {
	return
}

func (app *BaseApplication) SetOption(key string, value string) (log string) {
	return ""
}

func (app *BaseApplication) DeliverTx(tx []byte) Result {
	return NewResultOK(nil, "")
}

func (app *BaseApplication) CheckTx(tx []byte) Result {
	return NewResultOK(nil, "")
}

func (app *BaseApplication) Commit() Result {
	return NewResultOK([]byte("nil"), "")
}

func (app *BaseApplication) Query(req RequestQuery) (resQuery ResponseQuery) {
	return
}

func (app *BaseApplication) InitChain(req RequestInitChain) {
}

func (app *BaseApplication) BeginBlock(req RequestBeginBlock) {
}

func (app *BaseApplication) EndBlock(height uint64) (resEndBlock ResponseEndBlock) {
	return
}
