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

package-debian-tendermint: package-tendermint
	$(info Packaging Debian tendermint version $(tendermint_version)-$(BUILD_NUMBER))
	rm -rf BUILD/tendermint-$(tendermint_version)-$(BUILD_NUMBER) tmp/_gpg tmp/_gpgbuilder tmp/debian-binary tmp/control.tar.gz tmp/data.tar.xz
	cp -r BUILD/tendermint-$(tendermint_version) BUILD/tendermint-$(tendermint_version)-$(BUILD_NUMBER)

	cp -r extrafiles/DEBIAN BUILD/tendermint-$(tendermint_version)-$(BUILD_NUMBER)/
	sed -i "s/@TENDERMINT_VERSION@/$(tendermint_version)-$(BUILD_NUMBER)/" BUILD/tendermint-$(tendermint_version)-$(BUILD_NUMBER)/DEBIAN/changelog
	sed -i "s/@STABILITY@/stable/" BUILD/tendermint-$(tendermint_version)-$(BUILD_NUMBER)/DEBIAN/changelog
	sed -i "s/@DATETIMESTAMP@/`date +%a,\ %d\ %b\ %Y\ %T\ %z`/" BUILD/tendermint-$(tendermint_version)-$(BUILD_NUMBER)/DEBIAN/changelog
	sed -i "s/@TENDERMINT_VERSION@/$(tendermint_version)-$(BUILD_NUMBER)/" BUILD/tendermint-$(tendermint_version)-$(BUILD_NUMBER)/DEBIAN/control

	rm -rf BUILD/tendermint-$(tendermint_version)-$(BUILD_NUMBER)/usr/share/licenses
	mkdir -p BUILD/tendermint-$(tendermint_version)-$(BUILD_NUMBER)/usr/share/doc/tendermint
	cp BUILD/tendermint-$(tendermint_version)-$(BUILD_NUMBER)/DEBIAN/copyright BUILD/tendermint-$(tendermint_version)-$(BUILD_NUMBER)/usr/share/doc/tendermint
	gzip -c BUILD/tendermint-$(tendermint_version)-$(BUILD_NUMBER)/DEBIAN/changelog > BUILD/tendermint-$(tendermint_version)-$(BUILD_NUMBER)/usr/share/doc/tendermint/changelog.Debian.gz
	gzip -c BUILD/tendermint-$(tendermint_version)-$(BUILD_NUMBER)/DEBIAN/changelog > BUILD/tendermint-$(tendermint_version)-$(BUILD_NUMBER)/usr/share/doc/tendermint/changelog.Debian.amd64.gz

	sed -i "s/@INSTALLEDSIZE@/`du -ks BUILD/tendermint-$(tendermint_version)-$(BUILD_NUMBER) | cut -f 1`/" BUILD/tendermint-$(tendermint_version)-$(BUILD_NUMBER)/DEBIAN/control
	cd BUILD/tendermint-$(tendermint_version)-$(BUILD_NUMBER) && tar --owner=root --group=root -cvJf ../../tmp/data.tar.xz --exclude DEBIAN *
	cd BUILD/tendermint-$(tendermint_version)-$(BUILD_NUMBER)/DEBIAN && tar --owner=root --group=root -cvzf ../../../tmp/control.tar.gz *
	echo "2.0" > tmp/debian-binary

	cp extrafiles/_gpg tmp/
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
	ar r tmp/tendermint-$(tendermint_version)-$(BUILD_NUMBER)_amd64.deb tmp/debian-binary tmp/control.tar.gz tmp/data.tar.xz tmp/_gpgbuilder
	mv tmp/tendermint-$(tendermint_version)-$(BUILD_NUMBER)_amd64.deb RPMS/
	rm tmp/debian-binary tmp/control.tar.gz tmp/data.tar.xz tmp/_gpgbuilder tmp/_gpg

install-%:
#Make sure your host has the IAM role to read/write the S3 bucket OR that you set up ~/.boto
	aws s3 sync s3://tendermint-packages/centos/ tmp/s3/
	mkdir -p tmp/s3/7/os/x86_64/Packages
	cp -r RPMS/x86_64/$** tmp/s3/7/os/x86_64/Packages
	cp repofiles/RPM-GPG-KEY-Tendermint tmp/s3/7/os/x86_64/
	cp repofiles/tendermint.repo tmp/s3/7/os/x86_64/
	createrepo tmp/s3/7/os/x86_64/Packages -u http://tendermint-packages.s3-website-us-west-1.amazonaws.com/centos/7/os/x86_64/Packages -o tmp/s3/7/os/x86_64 --update -S --repo Tendermint --content tendermint --content basecoin --content ethermint
	rm -f tmp/s3/7/os/x86_64/repodata/repomd.xml.asc
	gpg --batch --passphrase "$(GPG_PASSPHRASE)" --sign -a tmp/s3/7/os/x86_64/repodata/repomd.xml
	aws s3 sync tmp/s3/ s3://tendermint-packages/centos/ --acl public-read

install-debian-tendermint: prepare-spec-tendermint
#Prereq: package-debian-tendermint
	aws s3 sync s3://tendermint-packages/debian/ tmp/debian-s3/
	mkdir -p tmp/debian-s3/pool tmp/debian-s3/dists/stable/main/binary-amd64
	cp RPMS/tendermint-$(tendermint_version)-$(BUILD_NUMBER)_amd64.deb tmp/debian-s3/pool
	cp repofiles/Release_amd64 tmp/debian-s3/dists/stable/main/binary-amd64/Release

#Package.gz	
	test -f tmp/debian-s3/dists/stable/main/binary-amd64/Packages.gz && gunzip tmp/debian-s3/dists/stable/main/binary-amd64/Packages.gz || rm -f tmp/debian-s3/dists/stable/main/binary-amd64/Package

	echo > tmp/Package
	echo "Filename: pool/tendermint-$(tendermint_version)-$(BUILD_NUMBER)_amd64.deb" >> tmp/Package
	echo "MD5sum: `md5sum RPMS/tendermint-$(tendermint_version)-$(BUILD_NUMBER)_amd64.deb | cut -d\  -f 1`" >> tmp/Package
	echo "SHA1: `sha1sum RPMS/tendermint-$(tendermint_version)-$(BUILD_NUMBER)_amd64.deb | cut -d\  -f 1`" >> tmp/Package
	echo "SHA256: `sha256sum RPMS/tendermint-$(tendermint_version)-$(BUILD_NUMBER)_amd64.deb | cut -d\  -f 1`" >> tmp/Package
	echo "Size: `stat -c %s RPMS/tendermint-$(tendermint_version)-$(BUILD_NUMBER)_amd64.deb | cut -d\  -f 1`" >> tmp/Package
	cat BUILD/tendermint-$(tendermint_version)-$(BUILD_NUMBER)/DEBIAN/control >> tmp/Package

	cat tmp/Package >> tmp/debian-s3/dists/stable/main/binary-amd64/Packages
	gzip tmp/debian-s3/dists/stable/main/binary-amd64/Packages
	rm -f tmp/Package

#Release / InRelease / Release.gpg
	cp repofiles/Release tmp/debian-s3/dists/stable
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

	aws s3 sync tmp/debian-s3/ s3://tendermint-packages/debian/ --acl public-read

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

