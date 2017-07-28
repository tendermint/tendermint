package common

import (
	"bytes"
	"fmt"
	"io/ioutil"
	"os"
	"testing"
	"time"
)

func TestWriteFileAtomic(t *testing.T) {
	data := []byte("Becatron")
	fname := fmt.Sprintf("/tmp/write-file-atomic-test-%v.txt", time.Now().UnixNano())
	err := WriteFileAtomic(fname, data, 0664)
	if err != nil {
		t.Fatal(err)
	}
	rData, err := ioutil.ReadFile(fname)
	if err != nil {
		t.Fatal(err)
	}
	if !bytes.Equal(data, rData) {
		t.Fatalf("data mismatch: %v != %v", data, rData)
	}
	if err := os.Remove(fname); err != nil {
		t.Fatal(err)
	}
}
