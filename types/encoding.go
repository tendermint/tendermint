package types

import proto "github.com/gogo/protobuf/proto"

func MarshalEvidence(eviI EvidenceI) ([]byte, error) {
	evi := &Evidence{}
	if err := evi.SetEvidenceI(eviI); err != nil {
		return nil, err
	}
	return proto.Marshal(evi)
}

func UnmarshalEvidence(bz []byte) (EvidenceI, error) {
	evi := &Evidence{}

	err := proto.Unmarshal(bz, evi)
	if err != nil {
		return nil, err
	}

	return evi.GetEvidenceI(), nil
}
