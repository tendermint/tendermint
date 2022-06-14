package types

// IsEmpty returns true if each list (default and threshold-recover) is empty otherwise false
func (m *VoteExtensions) IsEmpty() bool {
	if m == nil {
		return true
	}
	return len(m.Default) == 0 && len(m.ThresholdRecover) == 0
}
