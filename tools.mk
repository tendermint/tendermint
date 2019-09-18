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
# tools
###

TOOLS_DESTDIR  ?= $(GOPATH)/bin

CERTSTRAP     = $(TOOLS_DESTDIR)/certstrap
PROTOBUF     	= $(TOOLS_DESTDIR)/protoc
GOX						= $(TOOLS_DESTDIR)/gox
GOODMAN 			= $(TOOLS_DESTDIR)/goodman

all: tools

tools: certstrap protobuf gox goodman

check: check_tools

check_tools:
	@# https://stackoverflow.com/a/25668869
	@echo "Found tools: $(foreach tool,$(notdir $(GOTOOLS)),\
        $(if $(shell which $(tool)),$(tool),$(error "No $(tool) in PATH")))"

certstrap: $(CERTSTRAP)
$(CERTSTRAP):
	@echo "Get Certstrap"
	@go get github.com/square/certstrap@338204a88c4349b1c135eac1e8c14c693ad007da

protobuf: $(PROTOBUF)
$(PROTOBUF):
	@echo "Get Protobuf"
	## protobuf v1.3.0
	@go get github.com/gogo/protobuf/protoc-gen-gogo@0ca988a254f991240804bf9821f3450d87ccbb1b

gox: $(GOX)
$(GOX):
	@echo "Get Gox"
# used to build tm-monitor & tm-bench binaries
	## gox v1.0.1
	@go get github.com/mitchellh/gox@d8caaff5a9dc98f4cfa1fcce6e7265a04689f641

goodman: $(GOODMAN)
$(GOODMAN):
	@echo "Get Goodman"
	@go get github.com/snikch/goodman/cmd/goodman@10e37e294daa3c9a90abded60ff9924bafab3888

tools-clean:
	rm -f $(CERTSTRAP) $(PROTOBUF) $(GOX) $(GOODMAN)
	rm -f tools-stamp

.PHONY: all tools tools-clean
