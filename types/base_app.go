package types

type BaseApplication struct {
}

func NewBaseApplication() *BaseApplication {
	return &BaseApplication{}
}

func (app *BaseApplication) Info() (resInfo ResponseInfo) {
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

func (app *BaseApplication) Query(reqQuery RequestQuery) (resQuery ResponseQuery) {
	return
}

func (app *BaseApplication) InitChain(validators []*Validator) {
}

func (app *BaseApplication) BeginBlock(hash []byte, header *Header) {
}

func (app *BaseApplication) EndBlock(height uint64) (resEndBlock ResponseEndBlock) {
	return
}
