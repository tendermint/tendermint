package callgraph_test

import (
	"testing"

	"github.com/tendermint/tendermint/tools/panic/callgraph"
)

func TestStub(t *testing.T) {
	g := callgraph.New()
	g.ImportWithTests("github.com/tendermint/tendermint/internal/consensus")
	if err := g.Process(func(cg *callgraph.Triple) {
		if cg.Target.Name == "panic" {
			t.Logf("Panic call at %v", cg.Site)
		}
	}); err != nil {
		t.Fatal(err)
	}
}
