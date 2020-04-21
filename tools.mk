###
# Find OS and Go environment
# GO contains the Go binary
# FS contains the OS file separator
###
ifeq ($(OS),Windows_NT)
  GO := $(shell where go.exe 2> NUL)
  FS := "\\"
else
  GO := $(shell command -v go 2> /dev/null)
  FS := "/"
endif

ifeq ($(GO),)
  $(error could not find go. Is it in PATH? $(GO))
endif

GOPATH ?= $(shell $(GO) env GOPATH)
GITHUBDIR := $(GOPATH)$(FS)src$(FS)github.com

###
# Functions
###

go_get = $(if $(findstring Windows_NT,$(OS)),\
IF NOT EXIST $(GITHUBDIR)$(FS)$(1)$(FS) ( mkdir $(GITHUBDIR)$(FS)$(1) ) else (cd .) &\
IF NOT EXIST $(GITHUBDIR)$(FS)$(1)$(FS)$(2)$(FS) ( cd $(GITHUBDIR)$(FS)$(1) && git clone https://github.com/$(1)/$(2) ) else (cd .) &\
,\
mkdir -p $(GITHUBDIR)$(FS)$(1) &&\
(test ! -d $(GITHUBDIR)$(FS)$(1)$(FS)$(2) && cd $(GITHUBDIR)$(FS)$(1) && git clone https://github.com/$(1)/$(2)) || true &&\
)\
cd $(GITHUBDIR)$(FS)$(1)$(FS)$(2) && git fetch origin && git checkout -q $(3)

mkfile_path := $(abspath $(lastword $(MAKEFILE_LIST)))
mkfile_dir := $(shell cd $(shell dirname $(mkfile_path)); pwd)

###
# Go tools
###

TOOLS_DESTDIR  ?= $(GOPATH)/bin

CERTSTRAP     = $(TOOLS_DESTDIR)/certstrap
PROTOBUF     	= $(TOOLS_DESTDIR)/protoc
GOODMAN 			= $(TOOLS_DESTDIR)/goodman

all: tools
.PHONY: all

tools: certstrap protobuf goodman
.PHONY: tools

check: check_tools
.PHONY: check

check_tools:
	@# https://stackoverflow.com/a/25668869
	@echo "Found tools: $(foreach tool,$(notdir $(GOTOOLS)),\
        $(if $(shell which $(tool)),$(tool),$(error "No $(tool) in PATH")))"
.PHONY: check_tools

certstrap: $(CERTSTRAP)
$(CERTSTRAP):
	@echo "Get Certstrap"
	@go get github.com/square/certstrap@v1.2.0
.PHONY: certstrap

protobuf: $(PROTOBUF)
$(PROTOBUF):
	@echo "Get GoGo Protobuf"
	@go get github.com/gogo/protobuf/protoc-gen-gogo@v1.3.1
.PHONY: protobuf

goodman: $(GOODMAN)
$(GOODMAN):
	@echo "Get Goodman"
	@go get github.com/snikch/goodman/cmd/goodman@10e37e294daa3c9a90abded60ff9924bafab3888
.PHONY: goodman

tools-clean:
	rm -f $(CERTSTRAP) $(PROTOBUF) $(GOX) $(GOODMAN)
	rm -f tools-stamp
	rm -rf /usr/local/include/google/protobuf
	rm -f /usr/local/bin/protoc
.PHONY: tooks-clean

###
# Non Go tools
###

# Choose protobuf binary based on OS (only works for 64bit Linux and Mac).
# NOTE: On Mac, installation via brew (brew install protoc) might be favorable.
PROTOC_ZIP=""
ifneq ($(OS),Windows_NT)
		UNAME_S := $(shell uname -s)
		ifeq ($(UNAME_S),Linux)
			PROTOC_ZIP="protoc-3.10.1-linux-x86_64.zip"
		endif
		ifeq ($(UNAME_S),Darwin)
			PROTOC_ZIP="protoc-3.10.1-osx-x86_64.zip"
		endif
endif

protoc:
	@echo "Get Protobuf"
	@echo "In case of any errors, please install directly from https://github.com/protocolbuffers/protobuf/releases"
	@curl -OL https://github.com/protocolbuffers/protobuf/releases/download/v3.10.1/$(PROTOC_ZIP)
	@unzip -o $(PROTOC_ZIP) -d /usr/local bin/protoc
	@unzip -o $(PROTOC_ZIP) -d /usr/local 'include/*'
	@rm -f $(PROTOC_ZIP)
.PHONY: protoc
