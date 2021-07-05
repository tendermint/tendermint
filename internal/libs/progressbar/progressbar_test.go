package progressbar_test

import (
	"testing"
	"time"

	"github.com/tendermint/tendermint/internal/libs/progressbar"
)

func TestProgressBar(t *testing.T) {
	t.Skip()
	var bar progressbar.Bar
	bar.NewOption(0, 100)

	for i := 0; i <= 100; i++ {
		time.Sleep(5 * time.Millisecond)
		bar.Play(int64(i))
	}
	bar.Finish()
}

func TestProgressBarWithGraph(t *testing.T) {
	t.Skip()
	var bar progressbar.Bar
	bar.NewOptionWithGraph(0, 100, "#")
	for i := 0; i <= 100; i++ {
		time.Sleep(5 * time.Millisecond)
		bar.Play(int64(i))
	}
	bar.Finish()
}
