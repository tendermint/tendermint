package types

type ParamsEcho struct {
	Message string `json:"message,omitempty"`
}

type ParamsFlush struct {
}

type ParamsInfo struct {
	Version string `json:"version,omitempty"`
}

func ToParamsInfo(req RequestInfo) ParamsInfo {
	return ParamsInfo{
		Version: req.Version,
	}
}

type ParamsSetOption struct {
	Key   string `json:"key,omitempty"`
	Value string `json:"value,omitempty"`
}

func ToParamsSetOption(req RequestSetOption) ParamsSetOption {
	return ParamsSetOption{
		Key:   req.Key,
		Value: req.Value,
	}
}

type ParamsInitChain struct {
	Validators   []Validator `json:"validators"`
	GenesisBytes []byte      `json:"genesis_bytes,omitempty"`
}

func ToParamsInitChain(req RequestInitChain) ParamsInitChain {
	return ParamsInitChain{
		Validators:   req.Validators,
		GenesisBytes: req.GenesisBytes,
	}
}

type ParamsQuery struct {
	Data   []byte `json:"data,omitempty"`
	Path   string `json:"path,omitempty"`
	Height int64  `json:"height,omitempty"`
	Prove  bool   `json:"prove,omitempty"`
}

func ToParamsQuery(req RequestQuery) ParamsQuery {
	return ParamsQuery{
		Data:   req.Data,
		Path:   req.Path,
		Height: req.Height,
		Prove:  req.Prove,
	}
}

type ParamsBeginBlock struct {
	Hash                []byte             `json:"hash,omitempty"`
	Header              Header             `json:"header"`
	Validators          []SigningValidator `json:"validators,omitempty"`
	ByzantineValidators []Evidence         `json:"byzantine_validators"`
}

func ToParamsBeginBlock(req RequestBeginBlock) ParamsBeginBlock {
	vals := make([]SigningValidator, len(req.Validators))
	for i := 0; i < len(vals); i++ {
		v := req.Validators[i]
		vals[i] = *v
	}
	return ParamsBeginBlock{
		Hash:                req.Hash,
		Header:              req.Header,
		Validators:          vals,
		ByzantineValidators: req.ByzantineValidators,
	}
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

func ToParamsEndBlock(req RequestEndBlock) ParamsEndBlock {
	return ParamsEndBlock{
		Height: req.Height,
	}
}

type ParamsCommit struct {
}
