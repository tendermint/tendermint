package consensus

import (
	"io/ioutil"
	"os"
	"path"
	"strings"
	"testing"
)

var testTxt = `{"time":"2016-01-16T04:42:00.390Z","msg":[1,{"height":28219,"round":0,"step":"RoundStepPrevote"}]}
{"time":"2016-01-16T04:42:00.390Z","msg":[2,{"msg":[20,{"ValidatorIndex":0,"Vote":{"height":28219,"round":0,"type":1,"block_hash":"67F9689F15BEC30BF311FB4C0C80C5E661AA44E0","block_parts_header":{"total":1,"hash":"DFFD4409A1E273ED61AC27CAF975F446020D5676"},"signature":"4CC6845A128E723A299B470CCBB2A158612AA51321447F6492F3DA57D135C27FCF4124B3B19446A248252BDA45B152819C76AAA5FD35E1C07091885CE6955E05"}}],"peer_key":""}]}
{"time":"2016-01-16T04:42:00.392Z","msg":[1,{"height":28219,"round":0,"step":"RoundStepPrecommit"}]}
{"time":"2016-01-16T04:42:00.392Z","msg":[2,{"msg":[20,{"ValidatorIndex":0,"Vote":{"height":28219,"round":0,"type":2,"block_hash":"67F9689F15BEC30BF311FB4C0C80C5E661AA44E0","block_parts_header":{"total":1,"hash":"DFFD4409A1E273ED61AC27CAF975F446020D5676"},"signature":"1B9924E010F47E0817695DFE462C531196E5A12632434DE12180BBA3EFDAD6B3960FDB9357AFF085EB61729A7D4A6AD8408555D7569C87D9028F280192FD4E05"}}],"peer_key":""}]}
{"time":"2016-01-16T04:42:00.393Z","msg":[1,{"height":28219,"round":0,"step":"RoundStepCommit"}]}
{"time":"2016-01-16T04:42:00.395Z","msg":[1,{"height":28220,"round":0,"step":"RoundStepNewHeight"}]}`

func TestSeek(t *testing.T) {
	f, err := ioutil.TempFile(os.TempDir(), "seek_test_")
	if err != nil {
		t.Fatal(err)
	}

	stat, _ := f.Stat()
	name := stat.Name()

	_, err = f.WriteString(testTxt)
	if err != nil {
		t.Fatal(err)
	}
	f.Close()

	wal, err := NewWAL(path.Join(os.TempDir(), name), config.GetBool("cswal_light"))
	if err != nil {
		t.Fatal(err)
	}

	keyWord := "Precommit"
	n, err := wal.SeekFromEnd(func(b []byte) bool {
		if strings.Contains(string(b), keyWord) {
			return true
		}
		return false
	})
	if err != nil {
		t.Fatal(err)
	}

	// confirm n
	spl := strings.Split(testTxt, "\n")
	var i int
	var s string
	for i, s = range spl {
		if strings.Contains(s, keyWord) {
			break
		}
	}
	// n is lines from the end.
	spl = spl[i:]
	if n != len(spl) {
		t.Fatalf("Wrong nLines. Got %d, expected %d", n, len(spl))
	}

	b, err := ioutil.ReadAll(wal.fp)
	if err != nil {
		t.Fatal(err)
	}
	// first char is a \n
	spl2 := strings.Split(strings.Trim(string(b), "\n"), "\n")
	for i, s := range spl {
		if s != spl2[i] {
			t.Fatalf("Mismatch. Got %s, expected %s", spl2[i], s)
		}
	}

}
