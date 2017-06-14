##
# Extra checks, because we do not use autoconf. Set extra_check to false if it is bothering you.
##

extra_check = true
go_min_version = 1.8.3
gpg_key = 2122CBE9

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
gpg_check := $(shell gpg -K | grep '/$(gpg_key) ' | sed 's,^.*/\($(gpg_key)\) .*$$,\1,')
ifneq ($(gpg_check),$(gpg_key))
$(error GPG key $(gpg_key) not found.)
else
$(info GPG key $(gpg_key) found)
endif
ifndef GPG_PASSPHRASE
$(error GPG_PASSPHRASE not set)
endif
endif

###
# Here comes the real deal
###

binaries = tendermint basecoin ethermint
build-binaries = build-tendermint build-basecoin build-ethermint
package-binaries = package-tendermint package-basecoin package-ethermint
install-binaries = install-tendermint install-basecoin install-ethermint

all: $(binaries)
build: $(build-binaries)
package: $(package-binaries)
install: $(install-binaries)
$(binaries): %: build-% package-% install-% ;

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

package-tendermint: prepare-spec-tendermint
	$(info Packaging tendermint version $(tendermint_version))
	rpmbuild -bb SPECS/tendermint.spec
	./sign RPMS/x86_64/tendermint-$(tendermint_version)-$$BUILD_NUMBER.x86_64.rpm
	rpm -Kv RPMS/x86_64/tendermint-$(tendermint_version)-$$BUILD_NUMBER.x86_64.rpm

package-basecoin: prepare-spec-basecoin
	$(info Packaging basecoin version $(basecoin_version))
	rpmbuild -bb SPECS/basecoin.spec
	./sign RPMS/x86_64/basecoin-$(basecoin_version)-$$BUILD_NUMBER.x86_64.rpm
	rpm -Kv RPMS/x86_64/basecoin-$(basecoin_version)-$$BUILD_NUMBER.x86_64.rpm

package-ethermint: prepare-spec-ethermint ;
	$(info Packaging ethermint version $(ethermint_version))
	rpmbuild -bb SPECS/ethermint.spec
	./sign RPMS/x86_64/ethermint-$(ethermint_version)-$$BUILD_NUMBER.x86_64.rpm
	rpm -Kv RPMS/x86_64/ethermint-$(ethermint_version)-$$BUILD_NUMBER.x86_64.rpm

install-%:
#Make sure your host has the IAM role to read/write the S3 bucket OR that you set up ~/.boto
	aws s3 sync s3://tendermint-packages/centos/ tmp/s3/
	mkdir -p tmp/s3/7/cr/x86_64/Packages
	cp -r RPMS/x86_64/$** tmp/s3/7/cr/x86_64/Packages
	cp ./RPM-GPG-KEY-Tendermint tmp/s3/7/os/x86_64/
	cp ./tendermint.repo tmp/s3/7/os/x86_64/
	cp ./tendermint-dev.repo tmp/s3/7/cr/x86_64/
	createrepo tmp/s3/7/cr/x86_64/Packages -u http://tendermint-packages.s3-website-us-west-1.amazonaws.com/centos/7/cr/x86_64/Packages -o tmp/s3/7/cr/x86_64 --update -S --repo Tendermint --content tendermint --content basecoin --content ethermint
	rm -f tmp/s3/7/cr/x86_64/repodata/repomd.xml.asc
#TODO: make this more secure
	gpg --batch --passphrase "$(GPG_PASSPHRASE)" --clearsign tmp/s3/7/cr/x86_64/repodata/repomd.xml
	aws s3 sync tmp/s3/ s3://tendermint-packages/centos/ --acl public-read

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

