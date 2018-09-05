test_gcc:
	CGO_CXXFLAGS_ALLOW="(-fno-builtin-memcmp|-lpthread)" CGO_CFLAGS_ALLOW="-fno-builtin-memcmp" go test -tags gcc

.PHONY: test_gcc
