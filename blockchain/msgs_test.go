package blockchain

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestBlockRequestMessageValidateBasic(t *testing.T) {
	testCases := []struct {
		testName      string
		requestHeight int64
		expectErr     bool
	}{
		{"Valid Request Message", 0, false},
		{"Valid Request Message", 1, false},
		{"Invalid Request Message", -1, true},
	}

	for _, tc := range testCases {
		tc := tc
		t.Run(tc.testName, func(t *testing.T) {
			request := BlockRequestMessage{Height: tc.requestHeight}
			assert.Equal(t, tc.expectErr, request.ValidateBasic() != nil, "Validate Basic had an unexpected result")
		})
	}
}

func TestNoBlockResponseMessageValidateBasic(t *testing.T) {
	testCases := []struct {
		testName          string
		nonResponseHeight int64
		expectErr         bool
	}{
		{"Valid Non-Response Message", 0, false},
		{"Valid Non-Response Message", 1, false},
		{"Invalid Non-Response Message", -1, true},
	}

	for _, tc := range testCases {
		tc := tc
		t.Run(tc.testName, func(t *testing.T) {
			nonResponse := NoBlockResponseMessage{Height: tc.nonResponseHeight}
			assert.Equal(t, tc.expectErr, nonResponse.ValidateBasic() != nil, "Validate Basic had an unexpected result")
		})
	}
}

func TestStatusRequestMessageValidateBasic(t *testing.T) {
	testCases := []struct {
		testName      string
		requestHeight int64
		requestBase   int64
		expectErr     bool
	}{
		{"Valid Request Message", 0, 0, false},
		{"Valid Request Message", 1, 1, false},
		{"Invalid Request Message", -1, -1, true},
		{"Invalid Request Message", 1, 0, true},
	}

	for _, tc := range testCases {
		tc := tc
		t.Run(tc.testName, func(t *testing.T) {
			request := StatusRequestMessage{Height: tc.requestHeight, Base: tc.requestBase}
			assert.Equal(t, tc.expectErr, request.ValidateBasic() != nil, "Validate Basic had an unexpected result")
		})
	}
}

func TestStatusResponseMessageValidateBasic(t *testing.T) {
	testCases := []struct {
		testName       string
		responseHeight int64
		responseBase   int64
		expectErr      bool
	}{
		{"Valid Response Message", 0, 0, false},
		{"Valid Response Message", 1, 1, false},
		{"Invalid Response Message", -1, -1, true},
		{"Invalid Response Message", 1, 2, true},
	}

	for _, tc := range testCases {
		tc := tc
		t.Run(tc.testName, func(t *testing.T) {
			response := StatusResponseMessage{Height: tc.responseHeight, Base: tc.responseBase}
			assert.Equal(t, tc.expectErr, response.ValidateBasic() != nil, "Validate Basic had an unexpected result")
		})
	}
}
