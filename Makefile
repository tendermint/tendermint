PACKAGES=$(shell go list ./... | grep -v "test")

all: get_deps test

test:
	@echo "--> Running go test --race"
	@go test --race $(PACKAGES)
	@echo "--> Running integration tests"
	@bash ./test/integration_test.sh

get_deps:
	@echo "--> Running go get"
	@go get -v -d $(PACKAGES)

.PHONY: all test get_deps
