##
# Extra checks, because we do not use autoconf. Set extra_check to false if it is bothering you.
##

extra_check = true
go_min_version = 1.8.3

ifeq ($(extra_check),true)
ifndef GOPATH
$(error GOPATH not set)
else
go_version := $(shell go version | sed "s/^.* go\([0-9\.]*\) .*$$/\1/" )
$(info Found go version $(go_version))
go_version_check := $(shell echo -e "$(go_min_version)\n$(go_version)" | sort -V | head -1)
ifneq ($(go_min_version),$(go_version_check))
$(error go version go_min_version or above is required)
endif
endif
endif

###
# Here comes the real deal
###

binaries = tendermint basecoin ethermint
build-binaries = build-tendermint build-basecoin build-ethermint
dist-binaries = dist-tendermint dist-basecoin dist-ethermint

all: $(binaries)
build: $(build-binaries)
dist: $(dist-binaries)
$(binaries): %: build-% dist-% ;

###
# Building the binaries is not in the spec file, because in the spec file you already need to know the version number
###

build-tendermint:
	$(info Building tendermint)
	go get -u github.com/tendermint/tendermint/cmd/tendermint

build-basecoin:
	$(info Building basecoin)
	go get -u github.com/tendermint/basecoin/cmd/basecoin
	$(info Building basecli)
	go get -u github.com/tendermint/basecoin/cmd/basecli

build-ethermint:
	$(info Building ethermint)
	go get -d -u github.com/tendermint/ethermint/cmd/ethermint
	$(MAKE) -C $(GOPATH)/src/github.com/tendermint/ethermint get_vendor_deps
	$(MAKE) -C $(GOPATH)/src/github.com/tendermint/ethermint build
	cp $(GOPATH)/src/github.com/tendermint/ethermint/build/ethermint $(GOPATH)/bin

prepare-spec-%: $(GOPATH)/bin/%
	$(info Preparing build for $*)
	if [ -z "$(BUILD_NUMBER)" ]; then echo "BUILD_NUMBER not set" ; false ; fi
	mkdir -p {SPECS,tmp}
	$(eval $*_version=$(shell $< version | cut -d- -f1 ))
	echo "Version: $($*_version)" > SPECS/$*.spec
	echo "Release: $(BUILD_NUMBER)" >> SPECS/$*.spec
	cat spectemplates/$*.spec >> SPECS/$*.spec

dist-tendermint: prepare-spec-tendermint
	$(info Packaging tendermint version $(tendermint_version))
	rpmbuild -bb SPECS/tendermint.spec #--buildroot=tmp #--sign
#TODO

dist-basecoin: prepare-spec-basecoin
	$(info Packaging basecoin version $(basecoin_version))
	rpmbuild -bb SPECS/basecoin.spec #--sign
#TODO

dist-ethermint: prepare-spec-ethermint ;
	$(info Packaging ethermint version $(ethermint_version))
	rpmbuild -bb SPECS/ethermint.spec #--sign
#TODO

mostlyclean:
	rm -rf {BUILDROOT,SOURCES,SPECS,SRPMS,tmp}

clean: mostlyclean
	rm -rf {BUILD,RPMS}

distclean: clean
	rm -rf $(GOPATH)/src/github.com/tendermint/tendermint
	rm -rf $(GOPATH)/src/github.com/tendermint/basecoin 
	rm -rf $(GOPATH)/src/github.com/tendermint/ethermint 
	rm -rf $(GOPATH)/bin/tendermint
	rm -rf $(GOPATH)/bin/basecoin
	rm -rf $(GOPATH)/bin/basecli
	rm -rf $(GOPATH)/bin/ethermint

.PHONY : clean

