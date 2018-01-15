GOTOOLS = \
	github.com/Masterminds/glide \
	github.com/gogo/protobuf/protoc-gen-gogo \
	github.com/gogo/protobuf/gogoproto
	# github.com/alecthomas/gometalinter.v2 \

GOTOOLS_CHECK = glide gometalinter.v2 protoc protoc-gen-gogo
INCLUDE = -I=. -I=${GOPATH}/src -I=${GOPATH}/src/github.com/gogo/protobuf/protobuf

all: check get_vendor_deps protoc build test install metalinter

check: check_tools

########################################
###  Build

protoc:
	## If you get the following error,
	## "error while loading shared libraries: libprotobuf.so.14: cannot open shared object file: No such file or directory"
	## See https://stackoverflow.com/a/25518702
	protoc $(INCLUDE) --gogo_out=plugins=grpc:. common/*.proto
	@echo "--> adding nolint declarations to protobuf generated files"
	@awk '/package common/ { print "//nolint: gas"; print; next }1' common/types.pb.go > common/types.pb.go.new
	@mv common/types.pb.go.new common/types.pb.go

build:
	# Nothing to build!

install:
	# Nothing to install!


########################################
### Tools & dependencies

check_tools:
	@# https://stackoverflow.com/a/25668869
	@echo "Found tools: $(foreach tool,$(GOTOOLS_CHECK),\
        $(if $(shell which $(tool)),$(tool),$(error "No $(tool) in PATH")))"

get_tools:
	@echo "--> Installing tools"
	go get -u -v $(GOTOOLS)
	# @gometalinter.v2 --install

get_protoc:
	@# https://github.com/google/protobuf/releases
	curl -L https://github.com/google/protobuf/releases/download/v3.4.1/protobuf-cpp-3.4.1.tar.gz | tar xvz && \
		cd protobuf-3.4.1 && \
		DIST_LANG=cpp ./configure && \
		make && \
		make install && \
		cd .. && \
		rm -rf protobuf-3.4.1

update_tools:
	@echo "--> Updating tools"
	@go get -u $(GOTOOLS)

get_vendor_deps:
	@rm -rf vendor/
	@echo "--> Running glide install"
	@glide install


########################################
### Testing

test:
	go test -tags gcc `glide novendor`

test100:
	@for i in {1..100}; do make test; done


########################################
### Formatting, linting, and vetting

fmt:
	@go fmt ./...

metalinter:
	@echo "==> Running linter"
	gometalinter.v2 --vendor --deadline=600s --disable-all  \
		--enable=deadcode \
		--enable=goconst \
		--enable=goimports \
		--enable=gosimple \
		--enable=ineffassign \
		--enable=megacheck \
		--enable=misspell \
		--enable=staticcheck \
		--enable=safesql \
		--enable=structcheck \
		--enable=unconvert \
		--enable=unused \
		--enable=varcheck \
		--enable=vetshadow \
		./...

		#--enable=maligned \
		#--enable=gas \
		#--enable=aligncheck \
		#--enable=dupl \
		#--enable=errcheck \
		#--enable=gocyclo \
		#--enable=golint \ <== comments on anything exported
		#--enable=gotype \
		#--enable=interfacer \
		#--enable=unparam \
		#--enable=vet \

metalinter_all:
	protoc $(INCLUDE) --lint_out=. types/*.proto
	gometalinter.v2 --vendor --deadline=600s --enable-all --disable=lll ./...


# To avoid unintended conflicts with file names, always add to .PHONY
# unless there is a reason not to.
# https://www.gnu.org/software/make/manual/html_node/Phony-Targets.html
.PHONY: check protoc build check_tools get_tools get_protoc update_tools get_vendor_deps test fmt metalinter metalinter_all
