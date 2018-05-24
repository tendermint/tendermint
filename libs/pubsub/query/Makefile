gen_query_parser:
	@go get github.com/pointlander/peg
	peg -inline -switch query.peg

fuzzy_test:
	@go get github.com/dvyukov/go-fuzz/go-fuzz
	@go get github.com/dvyukov/go-fuzz/go-fuzz-build
	go-fuzz-build github.com/tendermint/tendermint/libs/pubsub/query/fuzz_test
	go-fuzz -bin=./fuzz_test-fuzz.zip -workdir=./fuzz_test/output

.PHONY: gen_query_parser fuzzy_test
