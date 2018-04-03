##
# Extra checks, because we do not use autoconf.
##

requirements_check = true
gpg_check = false
go_min_version = 1.9.4
gpg_key = 2122CBE9

ifeq ($(requirements_check),true)
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
ifeq ($(gpg_check),true)
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
endif

###
# Here comes the real deal
###

binaries = tendermint basecoind ethermint gaia
build-binaries = build-tendermint build-basecoind build-ethermint build-gaia
package-rpm = package-rpm-tendermint package-rpm-basecoind package-rpm-ethermint package-rpm-gaia
install-rpm = install-rpm-tendermint install-rpm-basecoind install-rpm-ethermint install-rpm-gaia
package-deb = package-deb-tendermint package-deb-basecoind package-deb-ethermint package-deb-gaia
install-deb = install-deb-tendermint install-deb-basecoind install-deb-ethermint install-deb-gaia

all: $(binaries)
build: $(build-binaries)
package: $(package-rpm) $(package-deb)
install: $(install-rpm) $(install-deb)
$(binaries): %: build-% package-rpm-% package-deb-%

###
# Build the binaries
###

git-branch:
	$(eval GIT_BRANCH=$(shell echo $${GIT_BRANCH:-master}))

gopath-setup:
	test -d $(GOPATH) || mkdir -p $(GOPATH)
	test -d $(GOPATH)/bin || mkdir -p $(GOPATH)/bin
	test -d $(GOPATH)/src || mkdir -p $(GOPATH)/src

build-tendermint: git-branch gopath-setup
	@echo "*** Building tendermint"
	go get -d -u github.com/tendermint/tendermint/cmd/tendermint
	cd $(GOPATH)/src/github.com/tendermint/tendermint && git checkout "$(GIT_BRANCH)" && git pull
	export PATH=$(GOPATH)/bin:$(PATH) && $(MAKE) -C $(GOPATH)/src/github.com/tendermint/tendermint get_tools get_vendor_deps build
	cp $(GOPATH)/src/github.com/tendermint/tendermint/build/tendermint $(GOPATH)/bin
	@echo "*** Built tendermint"

build-ethermint: git-branch gopath-setup
	@echo "*** Building ethermint"
	go get -d -u github.com/tendermint/ethermint/cmd/ethermint
	cd $(GOPATH)/src/github.com/tendermint/ethermint && git checkout "$(GIT_BRANCH)" && git pull
	export PATH=$(GOPATH)/bin:$(PATH) && $(MAKE) -C $(GOPATH)/src/github.com/tendermint/ethermint get_vendor_deps build
	cp $(GOPATH)/src/github.com/tendermint/ethermint/build/ethermint $(GOPATH)/bin
	@echo "*** Built ethermint"

build-gaia: git-branch gopath-setup
	@echo "*** Building gaia"
	go get -d -u go github.com/cosmos/gaia || echo "Workaround for go downloads."
	cd $(GOPATH)/src/github.com/cosmos/gaia && git checkout "$(GIT_BRANCH)" && git pull
	export PATH=$(GOPATH)/bin:$(PATH) && $(MAKE) -C $(GOPATH)/src/github.com/cosmos/gaia get_vendor_deps install
	@echo "*** Built gaia"

build-basecoind: git-branch gopath-setup
	@echo "*** Building basecoind from cosmos-sdk"
	go get -d -u github.com/cosmos/cosmos-sdk/examples/basecoin/cmd/basecoind
	cd $(GOPATH)/src/github.com/cosmos/cosmos-sdk && git checkout "$(GIT_BRANCH)" && git pull
	export PATH=$(GOPATH)/bin:$(PATH) && $(MAKE) -C $(GOPATH)/src/github.com/cosmos/cosmos-sdk get_tools get_vendor_deps build
	cp $(GOPATH)/src/github.com/cosmos/cosmos-sdk/build/basecoind $(GOPATH)/bin/basecoind
	@echo "*** Built basecoind from cosmos-sdk"

###
# Prepare package files
###

# set app_version
version-%:
	@echo "Checking if binary exists"
	test -f $(GOPATH)/bin/$*
	@echo "BUILD_NUMBER is $(BUILD_NUMBER)"
	test -n "$(BUILD_NUMBER)"
	$(eval $*_version=$(shell $(GOPATH)/bin/$* version | head -1 | cut -d- -f1 | sed 's/^\(ethermint:\s*\|\)\(v\|\)//' | tr -d '\t ' ))

# set build_folder
folder-%: version-%
	$(eval build_folder=BUILD/$*-$($*_version)-$(BUILD_NUMBER))

# clean up folder structure for package files
prepare-files = rm -rf $(build_folder) && mkdir -p $(build_folder) && cp -r ./$(1)/* $(build_folder) && mkdir -p $(build_folder)/usr/bin && cp $(GOPATH)/bin/$(1) $(build_folder)/usr/bin

##
## Package customizations for the different applications
##

prepare-tendermint =
prepare-ethermint = mkdir -p $(build_folder)/etc/ethermint && \
	cp $(GOPATH)/src/github.com/tendermint/ethermint/setup/genesis.json $(build_folder)/etc/ethermint/genesis.json && \
	cp -r $(GOPATH)/src/github.com/tendermint/ethermint/setup/keystore $(build_folder)/etc/ethermint
prepare-gaia =
prepare-basecoind = cp $(GOPATH)/bin/basecoind $(build_folder)/usr/bin

###
# Package the binary for CentOS/RedHat (RPM) and Debian/Ubuntu (DEB)
###

# Depends on rpmbuild, sorry, this can only be built on CentOS/RedHat machines.
package-rpm-%: folder-%
	@echo "*** Packaging RPM $* version $($*_version)"

	$(call prepare-files,$*)
	$(call prepare-$*)

	rm -rf $(build_folder)/DEBIAN
	mkdir -p $(build_folder)/usr/share/licenses/$*
	cp ./LICENSE $(build_folder)/usr/share/licenses/$*/LICENSE
	chmod -Rf a+rX,u+w,g-w,o-w $(build_folder)

	mkdir -p {SPECS,tmp}

	./generate-spec $* spectemplates SPECS
	sed -i "s/@VERSION@/$($*_version)/" SPECS/$*.spec
	sed -i "s/@BUILD_NUMBER@/$(BUILD_NUMBER)/" SPECS/$*.spec
	sed -i "s/@PACKAGE_NAME@/$*/" SPECS/$*.spec

	rpmbuild -bb SPECS/$*.spec --define "_topdir `pwd`" --define "_tmppath `pwd`/tmp"
	./sign RPMS/x86_64/$*-$($*_version)-$(BUILD_NUMBER).x86_64.rpm "$(gpg_key)" "`which gpg`"
	rpm -Kv RPMS/x86_64/$*-$($*_version)-$(BUILD_NUMBER).x86_64.rpm || echo "rpm returns non-zero exist for some reason. ($?)"
	@echo "*** Packaged RPM $* version $($*_version)"

package-deb-%: folder-%
	@echo "*** Packaging DEB $* version $($*_version)-$(BUILD_NUMBER)"

	$(call prepare-files,$*)
	$(call prepare-$*)

	mkdir -p $(build_folder)/usr/share/doc/$*
	cp $(build_folder)/DEBIAN/copyright $(build_folder)/usr/share/doc/$*
	chmod -Rf a+rX,u+w,g-w,o-w $(build_folder)

	sed -i "s/@VERSION@/$($*_version)-$(BUILD_NUMBER)/" $(build_folder)/DEBIAN/changelog
	sed -i "s/@STABILITY@/stable/" $(build_folder)/DEBIAN/changelog
	sed -i "s/@DATETIMESTAMP@/`date +%a,\ %d\ %b\ %Y\ %T\ %z`/" $(build_folder)/DEBIAN/changelog
	sed -i "s/@VERSION@/$($*_version)-$(BUILD_NUMBER)/" $(build_folder)/DEBIAN/control

	gzip -c $(build_folder)/DEBIAN/changelog > $(build_folder)/usr/share/doc/$*/changelog.Debian.gz
	gzip -c $(build_folder)/DEBIAN/changelog > $(build_folder)/usr/share/doc/$*/changelog.Debian.amd64.gz
	sed -i "s/@INSTALLEDSIZE@/`du -ks $(build_folder) | cut -f 1`/" $(build_folder)/DEBIAN/control

	cd $(build_folder) && tar --owner=root --group=root -cvJf ../../tmp/data.tar.xz --exclude DEBIAN *
	cd $(build_folder)/DEBIAN && tar --owner=root --group=root -cvzf ../../../tmp/control.tar.gz *
	echo "2.0" > tmp/debian-binary

	cp ./_gpg tmp/
	cd tmp && sed -i "s/@DATETIMESTAMP@/`date +%a\ %b\ %d\ %T\ %Y`/" _gpg
	cd tmp && sed -i "s/@BINMD5@/`md5sum debian-binary | cut -d\  -f1`/" _gpg
	cd tmp && sed -i "s/@BINSHA1@/`sha1sum debian-binary | cut -d\  -f1`/" _gpg
	cd tmp && sed -i "s/@BINSIZE@/`stat -c %s debian-binary | cut -d\  -f1`/" _gpg
	cd tmp && sed -i "s/@CONMD5@/`md5sum control.tar.gz | cut -d\  -f1`/" _gpg
	cd tmp && sed -i "s/@CONSHA1@/`sha1sum control.tar.gz | cut -d\  -f1`/" _gpg
	cd tmp && sed -i "s/@CONSIZE@/`stat -c %s control.tar.gz | cut -d\  -f1`/" _gpg
	cd tmp && sed -i "s/@DATMD5@/`md5sum data.tar.xz | cut -d\  -f1`/" _gpg
	cd tmp && sed -i "s/@DATSHA1@/`sha1sum data.tar.xz | cut -d\  -f1`/" _gpg
	cd tmp && sed -i "s/@DATSIZE@/`stat -c %s data.tar.xz | cut -d\  -f1`/" _gpg
	gpg --batch --passphrase "$(GPG_PASSPHRASE)" --clearsign tmp/_gpg
	mv tmp/_gpg.asc tmp/_gpgbuilder
	ar r tmp/$*-$($*_version)-$(BUILD_NUMBER)_amd64.deb tmp/debian-binary tmp/control.tar.gz tmp/data.tar.xz tmp/_gpgbuilder
	mv tmp/$*-$($*_version)-$(BUILD_NUMBER)_amd64.deb RPMS/
	rm tmp/debian-binary tmp/control.tar.gz tmp/data.tar.xz tmp/_gpgbuilder tmp/_gpg
	@echo "*** Packaged DEB $* version $($*_version)-$(BUILD_NUMBER)"

install-rpm-%: version-%
#Make sure your host has the IAM role to read/write the S3 bucket OR that you set up ~/.boto
	@echo "*** Uploading $*-$($*_version)-$(BUILD_NUMBER).x86_64.rpm to AWS $(DEVOPS_PATH)CentOS repository"
	aws s3 sync s3://tendermint-packages/$(DEVOPS_PATH)centos/ tmp/s3/ --delete
	mkdir -p tmp/s3/7/os/x86_64/Packages
	cp RPMS/x86_64/$*-$($*_version)-$(BUILD_NUMBER).x86_64.rpm tmp/s3/7/os/x86_64/Packages
	cp ./RPM-GPG-KEY-Tendermint tmp/s3/7/os/x86_64/
	cp ./tendermint.repo tmp/s3/7/os/x86_64/
	rm -f tmp/s3/7/os/x86_64/repodata/*.bz2 tmp/s3/7/os/x86_64/repodata/*.gz tmp/s3/7/os/x86_64/repodata/repomd.xml.asc
	createrepo tmp/s3/7/os/x86_64/Packages -u https://tendermint-packages.interblock.io/$(DEVOPS_PATH)centos/7/os/x86_64/Packages -o tmp/s3/7/os/x86_64 --update -S --repo Tendermint --content tendermint --content basecoind --content ethermint
	gpg --batch --passphrase "$(GPG_PASSPHRASE)" --detach-sign -a tmp/s3/7/os/x86_64/repodata/repomd.xml
	aws s3 sync tmp/s3/ s3://tendermint-packages/$(DEVOPS_PATH)centos/ --delete --acl public-read
	@echo "*** Uploaded $* to AWS $(DEVOPS_PATH)CentOS repository"

install-deb-%: version-%
	@echo "*** Uploading $*-$($*_version)-$(BUILD_NUMBER)_amd64.deb to AWS $(DEVOPS_PATH)Debian repository"
	@echo "Testing if $*-$($*_version)-$(BUILD_NUMBER)_amd64.deb is already uploaded"
	test ! -f tmp/debian-s3/pool/$*-$($*_version)-$(BUILD_NUMBER)_amd64.deb
	aws s3 sync s3://tendermint-packages/$(DEVOPS_PATH)debian/ tmp/debian-s3/ --delete
	@echo "Testing if $*-$($*_version)-$(BUILD_NUMBER)_amd64.deb is already uploaded"
	test ! -f tmp/debian-s3/pool/$*-$($*_version)-$(BUILD_NUMBER)_amd64.deb
	cp ./tendermint.list tmp/debian-s3/
	mkdir -p tmp/debian-s3/pool tmp/debian-s3/dists/stable/main/binary-amd64
	cp RPMS/$*-$($*_version)-$(BUILD_NUMBER)_amd64.deb tmp/debian-s3/pool
	cp ./Release_amd64 tmp/debian-s3/dists/stable/main/binary-amd64/Release

	#Packages / Packages.gz	

	echo > tmp/Package
	echo "Filename: pool/$*-$($*_version)-$(BUILD_NUMBER)_amd64.deb" >> tmp/Package
	echo "MD5sum: `md5sum RPMS/$*-$($*_version)-$(BUILD_NUMBER)_amd64.deb | cut -d\  -f 1`" >> tmp/Package
	echo "SHA1: `sha1sum RPMS/$*-$($*_version)-$(BUILD_NUMBER)_amd64.deb | cut -d\  -f 1`" >> tmp/Package
	echo "SHA256: `sha256sum RPMS/$*-$($*_version)-$(BUILD_NUMBER)_amd64.deb | cut -d\  -f 1`" >> tmp/Package
	echo "Size: `stat -c %s RPMS/$*-$($*_version)-$(BUILD_NUMBER)_amd64.deb | cut -d\  -f 1`" >> tmp/Package
	cat BUILD/$*-$($*_version)-$(BUILD_NUMBER)/DEBIAN/control >> tmp/Package

	cat tmp/Package >> tmp/debian-s3/dists/stable/main/binary-amd64/Packages
	rm -f tmp/debian-s3/dists/stable/main/binary-amd64/Packages.gz
	gzip -c tmp/debian-s3/dists/stable/main/binary-amd64/Packages > tmp/debian-s3/dists/stable/main/binary-amd64/Packages.gz
	rm -f tmp/Package

	#main / Release / InRelease / Release.gpg

	cp ./Release tmp/debian-s3/dists/stable/main/Release
	rm -f tmp/debian-s3/dists/stable/main/InRelease
	rm -f tmp/debian-s3/dists/stable/main/Release.gpg

	echo "MD5Sum:" >> tmp/debian-s3/dists/stable/main/Release
	cd tmp/debian-s3/dists/stable/main && for f in `find . -type f | sed 's/^.\///'` ; do test "$$f" == "Release" && continue ; echo -n " " ; md5sum $$f | sed "s/  / `stat -c %s $$f` /" ; done >> Release
	echo "SHA1:" >> tmp/debian-s3/dists/stable/main/Release
	cd tmp/debian-s3/dists/stable/main && for f in `find . -type f | sed 's/^.\///'` ; do test "$$f" == "Release" && continue ; echo -n " " ; sha1sum $$f | sed "s/  / `stat -c %s $$f` /" ; done >> Release
	echo "SHA256:" >> tmp/debian-s3/dists/stable/main/Release
	cd tmp/debian-s3/dists/stable/main && for f in `find . -type f | sed 's/^.\///'` ; do test "$$f" == "Release" && continue ; echo -n " " ; sha256sum $$f | sed "s/  / `stat -c %s $$f` /" ; done >> Release

	gpg --batch --passphrase "$(GPG_PASSPHRASE)" --digest-algo SHA256 -b -a tmp/debian-s3/dists/stable/main/Release
	mv tmp/debian-s3/dists/stable/main/Release.asc tmp/debian-s3/dists/stable/main/Release.gpg
	gpg --batch --passphrase "$(GPG_PASSPHRASE)" --digest-algo SHA512 --clearsign tmp/debian-s3/dists/stable/main/Release
	mv tmp/debian-s3/dists/stable/main/Release.asc tmp/debian-s3/dists/stable/main/InRelease

	#stable / Release / InRelease / Release.gpg

	cp ./Release tmp/debian-s3/dists/stable/Release
	rm -f tmp/debian-s3/dists/stable/InRelease
	rm -f tmp/debian-s3/dists/stable/Release.gpg

	echo "MD5Sum:" >> tmp/debian-s3/dists/stable/Release
	cd tmp/debian-s3/dists/stable && for f in `find . -type f | sed 's/^.\///'` ; do test "$$f" == "Release" && continue ; echo -n " " ; md5sum $$f | sed "s/  / `stat -c %s $$f` /" ; done >> Release
	echo "SHA1:" >> tmp/debian-s3/dists/stable/Release
	cd tmp/debian-s3/dists/stable && for f in `find . -type f | sed 's/^.\///'` ; do test "$$f" == "Release" && continue ; echo -n " " ; sha1sum $$f | sed "s/  / `stat -c %s $$f` /" ; done >> Release
	echo "SHA256:" >> tmp/debian-s3/dists/stable/Release
	cd tmp/debian-s3/dists/stable && for f in `find . -type f | sed 's/^.\///'` ; do test "$$f" == "Release" && continue ; echo -n " " ; sha256sum $$f | sed "s/  / `stat -c %s $$f` /" ; done >> Release

	gpg --batch --passphrase "$(GPG_PASSPHRASE)" --digest-algo SHA256 -b -a tmp/debian-s3/dists/stable/Release
	mv tmp/debian-s3/dists/stable/Release.asc tmp/debian-s3/dists/stable/Release.gpg
	gpg --batch --passphrase "$(GPG_PASSPHRASE)" --digest-algo SHA512 --clearsign tmp/debian-s3/dists/stable/Release
	mv tmp/debian-s3/dists/stable/Release.asc tmp/debian-s3/dists/stable/InRelease

	aws s3 sync tmp/debian-s3/ s3://tendermint-packages/$(DEVOPS_PATH)debian/ --delete --acl public-read
	@echo "*** Uploaded $*-$($*_version)-$(BUILD_NUMBER)_amd64.deb to AWS $(DEVOPS_PATH)Debian repository"

mostlyclean:
	rm -rf {BUILDROOT,SOURCES,SPECS,SRPMS,tmp}

clean: mostlyclean
	rm -rf {BUILD,RPMS}

distclean: clean
	rm -rf $(GOPATH)/src/github.com/tendermint/tendermint
	rm -rf $(GOPATH)/src/github.com/cosmos/cosmos-sdk
	rm -rf $(GOPATH)/src/github.com/tendermint/ethermint 
	rm -rf $(GOPATH)/bin/tendermint
	rm -rf $(GOPATH)/bin/basecoind
	rm -rf $(GOPATH)/bin/ethermint
	rm -rf $(GOPATH)/bin/gaia

.PHONY : clean

