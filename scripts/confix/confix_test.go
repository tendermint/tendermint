package main_test

import (
	"bytes"
	"context"
	"strings"
	"testing"

	"github.com/creachadair/tomledit"
	"github.com/google/go-cmp/cmp"

	confix "github.com/tendermint/tendermint/scripts/confix"
)

func mustParseConfig(t *testing.T, path string) *tomledit.Document {
	doc, err := confix.LoadConfig(path)
	if err != nil {
		t.Fatalf("Loading config: %v", err)
	}
	return doc
}

func TestGuessConfigVersion(t *testing.T) {
	tests := []struct {
		path, want string
	}{
		{"testdata/non-config.toml", ""},
		{"testdata/v30-config.toml", ""},
		{"testdata/v31-config.toml", ""},
		{"testdata/v32-config.toml", "v0.32"},
		{"testdata/v33-config.toml", "v0.33"},
		{"testdata/v34-config.toml", "v0.34"},
		{"testdata/v35-config.toml", "v0.35"},
		{"testdata/v36-config.toml", "v0.36"},
	}
	for _, test := range tests {
		t.Run(test.path, func(t *testing.T) {
			got := confix.GuessConfigVersion(mustParseConfig(t, test.path))
			if got != test.want {
				t.Errorf("Wrong version: got %q, want %q", got, test.want)
			}
		})
	}
}

func TestApplyFixes(t *testing.T) {
	ctx := context.Background()

	t.Run("Unknown", func(t *testing.T) {
		err := confix.ApplyFixes(ctx, mustParseConfig(t, "testdata/v31-config.toml"))
		if err == nil || !strings.Contains(err.Error(), "cannot tell what Tendermint version") {
			t.Error("ApplyFixes succeeded, but should have failed for an unknown version")
		}
	})
	t.Run("TooOld", func(t *testing.T) {
		err := confix.ApplyFixes(ctx, mustParseConfig(t, "testdata/v33-config.toml"))
		if err == nil || !strings.Contains(err.Error(), "unable to update version v0.33 config") {
			t.Errorf("ApplyFixes: got %v, want version error", err)
		}
	})
	t.Run("OK", func(t *testing.T) {
		doc := mustParseConfig(t, "testdata/v34-config.toml")
		if err := confix.ApplyFixes(ctx, doc); err != nil {
			t.Fatalf("ApplyFixes: unexpected error: %v", err)
		}

		t.Run("Fixpoint", func(t *testing.T) {
			// Verify that reapplying fixes to the same config succeeds, and does not
			// make any additional changes.
			var before bytes.Buffer
			if err := tomledit.Format(&before, doc); err != nil {
				t.Fatalf("Formatting document: %v", err)
			}
			if err := confix.CheckValid(before.Bytes()); err != nil {
				t.Fatalf("Validating output: %v", err)
			}
			want := before.String()

			// Re-parse the output from the first round of transformations.
			doc2, err := tomledit.Parse(&before)
			if err != nil {
				t.Fatalf("Parsing fixed output: %v", err)
			}
			if err := confix.ApplyFixes(ctx, doc2); err != nil {
				t.Fatalf("ApplyFixes: unexpected error: %v", err)
			}

			var after bytes.Buffer
			if err := tomledit.Format(&after, doc2); err != nil {
				t.Fatalf("Formatting document: %v", err)
			}
			got := after.String()

			if diff := cmp.Diff(want, got); diff != "" {
				t.Errorf("Reapplied fixes changed something: (-want, +got)\n%s", diff)
			}
		})
	})
}
