package main

import (
	"bytes"
	"context"
	"encoding/binary"
	"encoding/json"
	"errors"
	"fmt"
	"net/url"
	"os"
	"sort"
	"strconv"
	"strings"
	"time"

	dbm "github.com/tendermint/tm-db"

	tmmath "github.com/tendermint/tendermint/libs/math"
	tmproto "github.com/tendermint/tendermint/proto/tendermint/types"
	rpc "github.com/tendermint/tendermint/rpc/client/http"
	"github.com/tendermint/tendermint/types"
)

var (
	threshold         = tmmath.Fraction{Numerator: 2, Denominator: 3}
	startHeight int64 = 1
	endHeight int64 = 0 // is interpreted as the head of the chain
	interval int64 = 1000

)

const dbDirName = "validators"

func main() {
	if err := run(); err != nil {
		fmt.Printf("ERROR: %s\n", err.Error())
		fmt.Printf(`
Usage: 
  collect <rpcEdnpoint> [startHeight] [endHeight] [interval] 
  	- retrieves all validator sets from a provided endpoint from startHeight to endHeight and stores them
  analyze [dbPath] [threshold] 
  	- calculates the average amount of blocks before the threshold of the validator set is met      
  range [dbPath] 
  	- returns the range of validator sets stored in the database               	
`)
	}
}

func run() error {
	if len(os.Args) < 2 {
		return errors.New("expected either a collect or analyze command")
	}
	var err error

	switch os.Args[1] {
	case "collect":
		if len(os.Args) < 3 {
			return errors.New("expected rpc endpoint to be provided")
		}

		rpcEndpoint := os.Args[2]
		_, err := url.Parse(rpcEndpoint)
		if err != nil {
			return fmt.Errorf("unable to parse rpc endpint: %w", err)
		}

		if len(os.Args) > 3 {
			startHeight, err = strconv.ParseInt(os.Args[3], 10, 64)
			if err != nil {
				return err
			}
		}

		if len(os.Args) > 4 {
			endHeight, err = strconv.ParseInt(os.Args[3], 10, 64)
			if err != nil {
				return err
			}
		}

		return Collect(rpcEndpoint, startHeight, endHeight, interval)
	case "analyze":

		var dbDir string
		if len(os.Args) > 2 {
			dbDir = os.Args[2]
		} else {
			dbDir, err = os.Getwd()
			if err != nil {
				return err
			}
		}

		if len(os.Args) > 3 {
			threshold, err = tmmath.ParseFraction(os.Args[3])
			if err != nil {
				return err
			}
		}

		return Analyze(dbDir, threshold)
	case "range":
		var dbDir string
		if len(os.Args) > 2 {
			dbDir = os.Args[2]
		} else {
			dbDir, err = os.Getwd()
			if err != nil {
				return err
			}
		}
		return Range(dbDir)

	default:
		return fmt.Errorf("unknown command %s", os.Args[1])
	}
}

func Collect(rpcEndpoint string, startHeight, endHeight, interval int64) error {
	ctx := context.Background()

	dbDir, err := os.Getwd()
	if err != nil {
		return err
	}
	db, err := dbm.NewGoLevelDB(dbDirName, dbDir)
	if err != nil {
		return err
	}
	defer db.Close()

	client, err := rpc.New(rpcEndpoint, "/websocket")
	if err != nil {
		return err
	}

	var lastvs *types.ValidatorSet
	height := startHeight
	lastPrintedHeight := startHeight
	fmt.Printf("Starting validator set collection, height: %d\n", height)
	for {
		subctx, cancel := context.WithTimeout(ctx, 15*time.Second)
		defer cancel()

		vs, err := getValidators(subctx, client, height)
		if err != nil {
			return fmt.Errorf("fetching validators at height %d: %w", height, err)
		}

		// we have reached the latest height
		if vs == nil {
			break
		}

		// we skip if there were not any changes to the validator set
		if lastvs != nil && validatorSetsAreEqual(lastvs, vs) {
			height++
			continue
		}

		heightKey := make([]byte, 8)
		binary.LittleEndian.PutUint64(heightKey, uint64(height))

		vsProto, err := vs.ToProto()
		if err != nil {
			return err
		}

		vsBytes, err := vsProto.Marshal()
		if err != nil {
			return err
		}

		if err := db.Set(heightKey, vsBytes); err != nil {
			return fmt.Errorf("failed to persist at height %d: %w", height, err)
		}

		if height > lastPrintedHeight+(100*interval) {
			fmt.Println(height)
			lastPrintedHeight = height
		}

		height += interval
		lastvs = vs

		// we've reached the specified end height
		if height == endHeight {
			break
		}

		// we do this to do one final loop at the end height
		if endHeight != 0 && height > endHeight {
			height = endHeight
		}
	}

	fmt.Printf("Finished at height %d\n", height)

	return nil
}

func getValidators(ctx context.Context, client *rpc.HTTP, height int64) (*types.ValidatorSet, error) {
	const maxPages = 100

	var (
		perPage = 100
		vals    = []*types.Validator{}
		page    = 1
		total   = -1
	)

	for len(vals) != total && page <= maxPages {
		res, err := client.Validators(ctx, &height, &page, &perPage)
		if err != nil {
			if strings.Contains(err.Error(), "EOF") {
				time.Sleep(1 * time.Second)
				continue
			}

			// if the endpoint does not have any more validator sets
			// we return nil
			if strings.Contains(err.Error(), "must be less than or equal to the current blockchain height") {
				return nil, nil
			}
			return nil, err
		}

		// Validate response.
		if len(res.Validators) == 0 {
			return nil, fmt.Errorf("validator set is empty (height: %d, page: %d, per_page: %d)",
				height, page, perPage)
		}
		if res.Total <= 0 {
			return nil, fmt.Errorf("total number of vals is <= 0: %d (height: %d, page: %d, per_page: %d)",
				res.Total, height, page, perPage)
		}

		total = res.Total
		vals = append(vals, res.Validators...)
		page++
	}

	valSet, err := types.ValidatorSetFromExistingValidators(vals)
	if err != nil {
		return nil, err
	}
	return valSet, nil
}

func Analyze(dbDir string, threshold tmmath.Fraction) error {
	db, err := dbm.NewGoLevelDB(dbDirName, dbDir)
	if err != nil {
		return err
	}
	defer db.Close()

	iter, err := db.Iterator(nil, nil)
	if err != nil {
		return err
	}
	defer iter.Close()

	fmt.Print("Loading validators...")

	// naively load everything into memory
	// WARNING: check the size of the db before running
	var (
		vals          = make(map[int64]*types.ValidatorSet)
		lowestHeight  int64
		highestHeight int64
	)
	for ; iter.Valid(); iter.Next() {
		height := int64(binary.LittleEndian.Uint64(iter.Key()))

		if lowestHeight == 0 || height < lowestHeight {
			lowestHeight = height
		}

		if highestHeight == 0 || height > highestHeight {
			highestHeight = height
		}

		var vsProto = new(tmproto.ValidatorSet)
		if err := vsProto.Unmarshal(iter.Value()); err != nil {
			return err
		}

		vs, err := types.ValidatorSetFromProto(vsProto)
		if err != nil {
			return err
		}

		vals[height] = vs
	}

	var ranges = make([]int64, 0)

	for height := lowestHeight; height < highestHeight; height++ {
		vs, ok := vals[height]
		if !ok {
			continue
		}

		for comparisonHeight := height + 1; comparisonHeight <= highestHeight; comparisonHeight++ {
			comparisonVS, ok := vals[comparisonHeight]
			if !ok {
				continue
			}

			if isValidatorSetChangeWithinThreshold(vs, comparisonVS, threshold) {
				continue
			}

			// store how many heights before there was a validator set that changed more than the threshold
			ranges = append(ranges, comparisonHeight-height)
			break
		}
		fmt.Printf("processed height %d\n", height)

		// the first height should find at least one range else we end early
		if height == lowestHeight && len(ranges) == 0 {
			fmt.Printf("validator set never varied from %d to %d\n", lowestHeight, highestHeight)
			return nil
		}
	}

	jsn, err := json.Marshal(map[string]interface{}{
		"min":  min(ranges),
		"max":  max(ranges),
		"mean": mean(ranges),
		"size": len(ranges),
	})
	if err != nil {
		return err
	}

	fmt.Println(jsn)

	return nil
}

func Range(dbDir string) error {
	db, err := dbm.NewGoLevelDB(dbDirName, dbDir)
	if err != nil {
		return err
	}

	iter, err := db.Iterator(nil, nil)
	if err != nil {
		return err
	}
	defer iter.Close()

	var (
		lowestHeight  int64
		highestHeight int64
		counter       int = 0
	)

	for ; iter.Valid(); iter.Next() {
		height := int64(binary.LittleEndian.Uint64(iter.Key()))

		if lowestHeight == 0 || height < lowestHeight {
			lowestHeight = height
		}

		if highestHeight == 0 || height > highestHeight {
			highestHeight = height
		}
		counter++
	}

	fmt.Printf("Database contains %d validators sets ranging from %d to %d\n", counter, lowestHeight, highestHeight)

	return nil
}

func validatorSetsAreEqual(v1, v2 *types.ValidatorSet) bool {
	if len(v1.Validators) != len(v2.Validators) {
		return false
	}

	sort.Sort(types.ValidatorsByVotingPower(v1.Validators))
	sort.Sort(types.ValidatorsByVotingPower(v2.Validators))

	for idx, val1 := range v1.Validators {
		val2 := v2.Validators[idx]
		if !bytes.Equal(val1.Address, val2.Address) || val1.VotingPower != val2.VotingPower {
			return false
		}
	}
	return true
}

func isValidatorSetChangeWithinThreshold(v1, v2 *types.ValidatorSet, threshold tmmath.Fraction) bool {
	votingPowerNeeded := v2.TotalVotingPower() * int64(threshold.Numerator) / int64(threshold.Denominator)
	tally := int64(0)
	for _, val := range v1.Validators {
		_, v := v2.GetByAddress(val.Address)
		if v == nil {
			continue
		}
		tally += v.VotingPower

		if tally >= votingPowerNeeded {
			return true
		}
	}

	return false
}

func min(arr []int64) int64 {
	min := arr[0]
	for _, value := range arr {
		if value < min {
			min = value
		}
	}
	return min
}

func max(arr []int64) int64 {
	max := arr[0]
	for _, value := range arr {
		if value > max {
			max = value
		}
	}
	return max
}

func mean(arr []int64) int64 {
	sum := int64(0)
	for _, value := range arr {
		sum += value
	}
	return int64(sum / int64(len(arr)))
}
