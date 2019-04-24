build:
	@sh -c "'$(CURDIR)/build.sh'"

push:
	@sh -c "'$(CURDIR)/push.sh'"

build_develop:
	docker build -t "pakula/prism:develop" -f Dockerfile.develop .

build_testing:
	docker build --tag pakula/testing -f ./Dockerfile.testing .

push_develop:
	docker push "pakula/prism:develop"

.PHONY: build build_develop push push_develop
