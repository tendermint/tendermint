package bicall_test

import (
	"log"
	"sort"
	"strings"
	"testing"

	"github.com/google/go-cmp/cmp"
	"github.com/tendermint/tendermint/tools/panic/bicall"
)

func TestParse(t *testing.T) {
	want := mustFindNeedles(testInput)
	sortByLocation(want)

	var got []needle
	err := bicall.Parse(strings.NewReader(testInput), "testinput.go", func(c bicall.Call) error {
		got = append(got, needle{
			Name: c.Name,
			Line: c.Site.Line,
			Col:  c.Site.Column - 1,
		})
		t.Logf("Found call site for %q at %v", c.Name, c.Site)

		// Verify that the indicator comment shows up attributed to the site.
		tag := "@" + c.Name
		if len(c.Comments) != 1 || c.Comments[0] != tag {
			t.Errorf("Wrong comment at %v: got %+q, want [%q]", c.Site, c.Comments, tag)
		}
		return nil
	})
	if err != nil {
		t.Fatalf("Parse unexpectedly failed: %v", err)
	}
	sortByLocation(got)

	if diff := cmp.Diff(want, got); diff != "" {
		t.Errorf("Call site mismatch: (-want, +got)\n%s", diff)
	}
}

// sortByLocation permutes ns in-place to be ordered by line and column.
// The specific ordering rule is not important; we just need a consistent order
// for comparison of test results.
func sortByLocation(ns []needle) {
	sort.Slice(ns, func(i, j int) bool {
		if ns[i].Line == ns[j].Line {
			return ns[i].Col < ns[j].Col
		}
		return ns[i].Line < ns[j].Line
	})
}

// To add call sites to the test, include a trailing line comment having the
// form "//@name", where "name" is a built-in function name.
// The first offset of that name on the line prior to the comment will become
// an expected call site for that function.
const testInput = `
package testinput
func main() {
   defer func() {
      x := recover() //@recover
      if x != nil {
         println("whoa") //@println
      }
   }()
   ip := new(int)                  //@new
   *ip = 3 + copy([]byte{}, "")    //@copy
   panic(fmt.Sprintf("ip=%p", ip)) //@panic
}
`

// A needle is a name at a location in the source, that is expected to be
// located in a scan of the input for built-in calls.
// N.B. Fields are exported to allow comparison by the cmp package.
type needle struct {
	Name      string
	Line, Col int
}

func mustFindNeedles(src string) []needle {
	var needles []needle
	for i, raw := range strings.Split(src, "\n") {
		tag := strings.SplitN(raw, "//@", 2)
		if len(tag) == 1 {
			continue // no needle on this line
		}
		name := strings.TrimSpace(tag[1])
		col := strings.Index(tag[0], name)
		if col < 0 {
			log.Panicf("No match for %q on line %d of test input", name, i+1)
		}
		needles = append(needles, needle{
			Name: name,
			Line: i + 1,
			Col:  col,
		})
	}
	return needles
}
