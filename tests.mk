#!/usr/bin/make -f

########################################
### Testing

BINDIR ?= $(GOPATH)/bin

## required to be run first by most tests
build_docker_test_image:
	docker build -t tester -f ./test/docker/Dockerfile .
.PHONY: build_docker_test_image

### coverage, app, persistence, and libs tests
test_cover:
	# run the go unit tests with coverage
	bash test/test_cover.sh
.PHONY: test_cover

test_apps:
	# run the app tests using bash
	# requires `abci-cli` and `tendermint` binaries installed
	bash test/app/test.sh
.PHONY: test_apps

test_abci_apps:
	bash abci/tests/test_app/test.sh
.PHONY: test_abci_apps

test_abci_cli:
	# test the cli against the examples in the tutorial at:
	# ./docs/abci-cli.md
	# if test fails, update the docs ^
	@ bash abci/tests/test_cli/test.sh
.PHONY: test_abci_cli

test_persistence:
	# run the persistence tests using bash
	# requires `abci-cli` installed
	docker run --name run_persistence -t tester bash test/persist/test_failure_indices.sh

	# TODO undockerize
	# bash test/persist/test_failure_indices.sh
.PHONY: test_persistence

test_p2p:
	docker rm -f rsyslog || true
	rm -rf test/logs && mkdir -p test/logs
	docker run -d -v "$(CURDIR)/test/logs:/var/log/" -p 127.0.0.1:5514:514/udp --name rsyslog voxxit/rsyslog
	# requires 'tester' the image from above
	bash test/p2p/test.sh tester
	# the `docker cp` takes a really long time; uncomment for debugging
	#
	# mkdir -p test/p2p/logs && docker cp rsyslog:/var/log test/p2p/logs
.PHONY: test_p2p

test_p2p_ipv6:
	# IPv6 tests require Docker daemon with IPv6 enabled, e.g. in daemon.json:
	#
	# {
	#	"ipv6": true,
	#   "fixed-cidr-v6": "2001:db8:1::/64"
	# }
	#
	# Docker for Mac can set this via Preferences -> Docker Engine.
	docker rm -f rsyslog || true
	rm -rf test/logs && mkdir -p test/logs
	docker run -d -v "$(CURDIR)/test/logs:/var/log/" -p 127.0.0.1:5514:514/udp --name rsyslog voxxit/rsyslog
	# requires 'tester' the image from above
	bash test/p2p/test.sh tester 6
	# the `docker cp` takes a really long time; uncomment for debugging
	#
	# mkdir -p test/p2p/logs && docker cp rsyslog:/var/log test/p2p/logs
.PHONY: test_p2p_ipv6

test_integrations:
	make build_docker_test_image
	make tools
	make install
	make test_cover
	make test_apps
	make test_abci_apps
	make test_abci_cli
	make test_libs
	make test_persistence
	make test_p2p
	# Disabled by default since it requires Docker daemon with IPv6 enabled
	#make test_p2p_ipv6
.PHONY: test_integrations

test_release:
	@go test -tags release $(PACKAGES)
.PHONY: test_release

test100:
	@for i in {1..100}; do make test; done
.PHONY: test100

vagrant_test:
	vagrant up
	vagrant ssh -c 'make test_integrations'
.PHONY: vagrant_test

### go tests
test:
	@echo "--> Running go test"
	@go test -p 1 $(PACKAGES)
.PHONY: test

test_race:
	@echo "--> Running go test --race"
	@go test -p 1 -v -race $(PACKAGES)
.PHONY: test_race

# uses https://github.com/sasha-s/go-deadlock/ to detect potential deadlocks
test_with_deadlock:
	make set_with_deadlock
	make test
	make cleanup_after_test_with_deadlock
.PHONY: test_with_deadlock

set_with_deadlock:
	@echo "Get Goid"
	@go get github.com/petermattis/goid@b0b1615b78e5ee59739545bb38426383b2cda4c9
	@echo "Get Go-Deadlock"
	@go get github.com/sasha-s/go-deadlock@d68e2bc52ae3291765881b9056f2c1527f245f1e
	find . -name "*.go" | grep -v "vendor/" | xargs -n 1 sed -i.bak 's/sync.RWMutex/deadlock.RWMutex/'
	find . -name "*.go" | grep -v "vendor/" | xargs -n 1 sed -i.bak 's/sync.Mutex/deadlock.Mutex/'
	find . -name "*.go" | grep -v "vendor/" | xargs -n 1 goimports -w
.PHONY: set_with_deadlock

# cleanes up after you ran test_with_deadlock
cleanup_after_test_with_deadlock:
	find . -name "*.go" | grep -v "vendor/" | xargs -n 1 sed -i.bak 's/deadlock.RWMutex/sync.RWMutex/'
	find . -name "*.go" | grep -v "vendor/" | xargs -n 1 sed -i.bak 's/deadlock.Mutex/sync.Mutex/'
	find . -name "*.go" | grep -v "vendor/" | xargs -n 1 goimports -w
	# cleans up the deps to not include the need libs
	go mod tidy
.PHONY: cleanup_after_test_with_deadlock
