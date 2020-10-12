all: docker runner

docker:
	docker build --tag tendermint/e2e-node -f docker/Dockerfile ../..

# We need to build support for database backends into the app in
# order to build a binary with a Tendermint node in it (for built-in
# ABCI testing).
app:
	go build -o build/app -tags badgerdb,boltdb,cleveldb,rocksdb ./app
	

BUILD_TAGS?=maverick
CGO_ENABLED ?= 0

# handle cleveldb
ifeq (cleveldb,$(findstring cleveldb,$(TENDERMINT_BUILD_OPTIONS)))
  CGO_ENABLED=1
  BUILD_TAGS += cleveldb
endif

# handle badgerdb
ifeq (badgerdb,$(findstring badgerdb,$(TENDERMINT_BUILD_OPTIONS)))
  BUILD_TAGS += badgerdb
endif

# handle rocksdb
ifeq (rocksdb,$(findstring rocksdb,$(TENDERMINT_BUILD_OPTIONS)))
  CGO_ENABLED=1
  BUILD_TAGS += rocksdb
endif

# handle boltdb
ifeq (boltdb,$(findstring boltdb,$(TENDERMINT_BUILD_OPTIONS)))
  BUILD_TAGS += boltdb
endif

maverick:
	CGO_ENABLED=$(CGO_ENABLED) go build -o build/maverick -mod=readonly -tags $(BUILD_TAGS) ./maverick

runner:
	go build -o build/runner ./runner

.PHONY: app docker maverick runner 
