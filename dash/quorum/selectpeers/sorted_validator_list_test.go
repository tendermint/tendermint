package selectpeers

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/tendermint/tendermint/dash/quorum/mock"
)

// Test_sortableValidatorList_Index checks if the sortableValidatorList.Index() works correctly
func Test_sortedValidatorList_Index(t *testing.T) {
	quorumHash := mock.NewQuorumHash(100)
	tests := []struct {
		name   string
		list   sortedValidatorList
		search sortableValidator
		want   int
	}{
		{
			name:   "miss",
			list:   newSortedValidatorList(mock.NewValidators(5), quorumHash),
			search: newSortableValidator(*mock.NewValidator(10), quorumHash),
			want:   -1,
		},
		{
			name:   "i=0",
			list:   newSortedValidatorList(mock.NewValidators(500), quorumHash),
			search: newSortableValidator(*mock.NewValidator(1), quorumHash),
			want:   0,
		},
		{
			name:   "i=4",
			list:   newSortedValidatorList(mock.NewValidators(500), mock.NewQuorumHash(20531)),
			search: newSortableValidator(*mock.NewValidator(4), mock.NewQuorumHash(20531)),
			want:   4,
		},
	}
	// nolint:scopelint
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := tt.list.index(tt.search)
			if tt.want < 0 {
				assert.Less(t, got, 0)
			} else {
				assert.GreaterOrEqual(t, got, 0, "not found: SearchKey=%x", tt.search.sortKey)
				assert.EqualValues(t, tt.search.ProTxHash, tt.list[got].ProTxHash, "invalid index: SearchKey=%x", tt.search.sortKey)
			}
		})
	}
}
