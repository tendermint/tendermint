GOTOOLS = \
					github.com/Masterminds/glide

tools:
	go get -v $(GOTOOLS)

get_deps: tools
	@echo "--> Running glide install"
	@glide install

.PHONY: get_deps
