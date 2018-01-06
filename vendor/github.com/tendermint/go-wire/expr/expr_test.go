package expr

import (
	"strings"
	"testing"

	cmn "github.com/tendermint/tmlibs/common"
)

func TestParse(t *testing.T) {
	testParse(t, `"foobar"`, `"foobar"`)
	testParse(t, "0x1234", "0x1234")
	testParse(t, "0xbeef", "0xBEEF")
	testParse(t, "xbeef", "xBEEF")
	testParse(t, "12345", "{i 12345}")
	testParse(t, "u64:12345", "{u64 12345}")
	testParse(t, "i64:12345", "{i64 12345}")
	testParse(t, "i64:-12345", "{i64 -12345}")
	testParse(t, "[1 u64:2]", "[{i 1},{u64 2}]")
	testParse(t, "[(1 2) (3 4)]", "[({i 1} {i 2}),({i 3} {i 4})]")
	testParse(t, "0x1234 1 u64:2 [3 4]", "(0x1234 {i 1} {u64 2} [{i 3},{i 4}])")
	testParse(t, "[(1 <sig:user1>)(2 <sig:user2>)][3 4]",
		"([({i 1} <sig:user1>),({i 2} <sig:user2>)] [{i 3},{i 4}])")
}

func testParse(t *testing.T, input string, expected string) {
	got, err := ParseReader(input, strings.NewReader(input))
	if err != nil {
		t.Error(err.Error())
		return
	}
	gotStr := cmn.Fmt("%v", got)
	if gotStr != expected {
		t.Error(cmn.Fmt("Expected %v, got %v", expected, gotStr))
	}
}

func TestBytes(t *testing.T) {
	testBytes(t, `"foobar"`, `0106666F6F626172`)
	testBytes(t, "0x1234", "01021234")
	testBytes(t, "0xbeef", "0102BEEF")
	testBytes(t, "xbeef", "BEEF")
	testBytes(t, "12345", "023039")
	testBytes(t, "u64:12345", "0000000000003039")
	testBytes(t, "i64:12345", "0000000000003039")
	testBytes(t, "i64:-12345", "FFFFFFFFFFFFCFC7")
	testBytes(t, "[1 u64:2]", "010201010000000000000002")
	testBytes(t, "[(1 2) (3 4)]", "01020101010201030104")
	testBytes(t, "0x1234 1 u64:2 [3 4]", "0102123401010000000000000002010201030104")
	testBytes(t, "[(1 <sig:user1>)(2 <sig:user2>)][3 4]",
		"0102010100010200010201030104")
}

func testBytes(t *testing.T, input string, expected string) {
	got, err := ParseReader(input, strings.NewReader(input))
	if err != nil {
		t.Error(err.Error())
		return
	}
	gotBytes, err := got.(Byteful).Bytes()
	if err != nil {
		t.Error(err.Error())
		return
	}
	gotHex := cmn.Fmt("%X", gotBytes)
	if gotHex != expected {
		t.Error(cmn.Fmt("Expected %v, got %v", expected, gotHex))
	}
}
