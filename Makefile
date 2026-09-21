# The bridge builds with the pure-Go Olm implementation, so no libolm system
# library is needed: every target passes -tags goolm.

.PHONY: build test acceptance acceptance-mutation

build:
	./scripts/build.sh

test:
	./scripts/test.sh

acceptance:
	./scripts/acceptance.sh

acceptance-mutation:
	./scripts/acceptance-mutation.sh
