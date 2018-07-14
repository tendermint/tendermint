gen_query_parser:
	go get -u -v github.com/pointlander/peg
	peg -inline -switch query.peg

fuzzy_test:
	go get -u -v github.com/dvyukov/go-fuzz/go-fuzz
	go get -u -v github.com/dvyukov/go-fuzz/go-fuzz-build
	go-fuzz-build github.com/tendermint/tendermint/libs/pubsub/query/fuzz_test
	go-fuzz -bin=./fuzz_test-fuzz.zip -workdir=./fuzz_test/output

.PHONY: gen_query_parser fuzzy_test
