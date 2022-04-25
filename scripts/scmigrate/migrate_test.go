package scmigrate

import (
	"bytes"
	"context"
	"math/rand"
	"testing"

	"github.com/gogo/protobuf/proto"
	dbm "github.com/tendermint/tm-db"

	"github.com/tendermint/tendermint/types"
)

func appendRandomMigrations(in []toMigrate, num int) []toMigrate {
	if in == nil {
		in = []toMigrate{}
	}

	for i := 0; i < num; i++ {
		height := rand.Int63()
		if height <= 0 {
			continue
		}
		in = append(in, toMigrate{commit: &types.Commit{Height: height}})
	}
	return in
}

func assertWellOrderedMigrations(t *testing.T, testData []toMigrate) {
	t.Run("ValuesDescend", func(t *testing.T) {
		for idx := range testData {
			height := testData[idx].commit.Height
			if idx == 0 {
				continue
			}
			prev := testData[idx-1].commit.Height
			if prev < height {
				t.Fatal("height decreased in sort order")
			}
		}
	})
	t.Run("EarliestIsZero", func(t *testing.T) {
		earliestHeight := testData[len(testData)-1].commit.Height
		if earliestHeight != 0 {
			t.Fatalf("the earliest height is not 0: %d", earliestHeight)
		}
	})
}

func getLatestHeight(data []toMigrate) int64 {
	var out int64

	for _, d := range data {
		if d.commit.Height >= out {
			out = d.commit.Height
		}
	}

	return out
}

func insertTestData(t *testing.T, db dbm.DB, data []toMigrate) {
	t.Helper()

	batch := db.NewBatch()

	for idx, val := range data {
		payload, err := proto.Marshal(val.commit.ToProto())
		if err != nil {
			t.Fatal(err)
		}

		if err := batch.Set(makeKeyFromPrefix(prefixSeenCommit, int64(idx)), payload); err != nil {
			t.Fatal(err)
		}
	}
	if err := batch.WriteSync(); err != nil {
		t.Fatal(err)
	}
	if err := batch.Close(); err != nil {
		t.Fatal(err)
	}
}

func TestMigrations(t *testing.T) {
	t.Run("Sort", func(t *testing.T) {
		t.Run("HandCraftedData", func(t *testing.T) {
			testData := []toMigrate{
				{commit: &types.Commit{Height: 100}},
				{commit: &types.Commit{Height: 0}},
				{commit: &types.Commit{Height: 8}},
				{commit: &types.Commit{Height: 1}},
			}

			sortMigrations(testData)
			assertWellOrderedMigrations(t, testData)
		})
		t.Run("RandomGeneratedData", func(t *testing.T) {
			testData := []toMigrate{{commit: &types.Commit{Height: 0}}}

			testData = appendRandomMigrations(testData, 10000)

			sortMigrations(testData)
			assertWellOrderedMigrations(t, testData)
		})
	})
	t.Run("InvalidMigrations", func(t *testing.T) {
		if _, err := makeToMigrate(nil); err == nil {
			t.Fatal("should error for nil migrations")
		}
		if _, err := makeToMigrate([]byte{}); err == nil {
			t.Fatal("should error for empty migrations")
		}
		if _, err := makeToMigrate([]byte("invalid")); err == nil {
			t.Fatal("should error for empty migrations")
		}
	})

	t.Run("GetSeenCommits", func(t *testing.T) {
		ctx, cancel := context.WithCancel(context.Background())
		defer cancel()

		db := dbm.NewMemDB()
		data := appendRandomMigrations([]toMigrate{}, 100)
		insertTestData(t, db, data)
		commits, err := getAllSeenCommits(ctx, db)
		if err != nil {
			t.Fatal(err)
		}
		if len(commits) != len(data) {
			t.Log("inputs", len(data))
			t.Log("commits", len(commits))
			t.Fatal("migrations not found in database")
		}
	})
	t.Run("Integration", func(t *testing.T) {
		ctx, cancel := context.WithCancel(context.Background())
		defer cancel()

		db := dbm.NewMemDB()
		data := appendRandomMigrations([]toMigrate{}, 1000)
		insertTestData(t, db, data)

		latestHeight := getLatestHeight(data)
		for _, test := range []string{"Migration", "Idempotency"} {
			// run the test twice to make sure that it's
			// safe to rerun
			t.Run(test, func(t *testing.T) {
				if err := Migrate(ctx, db); err != nil {
					t.Fatalf("Migration failed: %v", err)
				}

				post, err := getAllSeenCommits(ctx, db)
				if err != nil {
					t.Fatalf("Fetching seen commits: %v", err)
				}

				if len(post) != 1 {
					t.Fatalf("Wrong number of commits: got %d, wanted 1", len(post))
				}

				wantKey := makeKeyFromPrefix(prefixSeenCommit)
				if !bytes.Equal(post[0].key, wantKey) {
					t.Errorf("Seen commit key: got %x, want %x", post[0].key, wantKey)
				}
				if got := post[0].commit.Height; got != latestHeight {
					t.Fatalf("Wrong commit height after migration: got %d, wanted %d", got, latestHeight)
				}
			})
		}
	})

}
