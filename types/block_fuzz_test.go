package types

import (
	"testing"

	fuzz "github.com/google/gofuzz"
)

// Panics on trying to set interfaces
// https://github.com/google/gofuzz/issues/27
// func TestFuzzBlockProto_10000(t *testing.T) {
// 	fuzzer := fuzz.New()
// 	block := &Block{}

// 	for i := 0; i < 10000; i++ {
// 		fmt.Println(i)
// 		fuzzer.Fuzz(block)
// bp, err := block.ToProto()
// b := Block{}
// _ = b.FromProto(bp)
// 	}
// }

func TestFuzzCommitProto_10000(t *testing.T) {
	fuzzer := fuzz.New().NilChance(.5)
	commit := &Commit{}

	for i := 0; i < 10000; i++ {
		fuzzer.Fuzz(commit)
		cp := commit.ToProto()

		c := Commit{}
		_ = c.FromProto(cp)
	}
}

func TestFuzzDataProto_10000(t *testing.T) {
	fuzzer := fuzz.New()
	data := &Data{}

	for i := 0; i < 10000; i++ {
		fuzzer.Fuzz(data)
		dp := data.ToProto()

		d := Data{}
		d.FromProto(*dp)
	}
}

// Panics on trying to set interfaces
// https://github.com/google/gofuzz/issues/27
// func TestFuzzEvidenceProto_10000(t *testing.T) {
// 	fuzzer := fuzz.New()
// 	evidence := &EvidenceData{}

// 	for i := 0; i < 10000; i++ {
// 		fuzzer.Fuzz(evidence)
// 		ep, _ := evidence.ToProto()

// 		d := EvidenceData{}
// 		d.FromProto(*ep)
// 	}
// }
