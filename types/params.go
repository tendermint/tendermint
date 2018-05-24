package types

type ParamsEcho struct {
	Message string `json:"message,omitempty"`
}

type ParamsFlush struct {
}

type ParamsInfo struct {
	Version string `json:"version,omitempty"`
}

type ParamsSetOption struct {
	Key   string `json:"key,omitempty"`
	Value string `json:"value,omitempty"`
}

type ParamsInitChain struct {
	Validators   []Validator `json:"validators"`
	GenesisBytes []byte      `json:"genesis_bytes,omitempty"`
}

type ParamsQuery struct {
	Data   []byte `json:"data,omitempty"`
	Path   string `json:"path,omitempty"`
	Height int64  `json:"height,omitempty"`
	Prove  bool   `json:"prove,omitempty"`
}

type ParamsBeginBlock struct {
	Hash                []byte             `json:"hash,omitempty"`
	Header              Header             `json:"header"`
	Validators          []SigningValidator `json:"validators,omitempty"`
	ByzantineValidators []Evidence         `json:"byzantine_validators"`
}

type ParamsCheckTx struct {
	Tx []byte `json:"tx,omitempty"`
}

type ParamsDeliverTx struct {
	Tx []byte `json:"tx,omitempty"`
}

type ParamsEndBlock struct {
	Height int64 `json:"height,omitempty"`
}

type ParamsCommit struct {
}
