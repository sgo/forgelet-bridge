# The bridge builds with the pure-Go Olm implementation, so no libolm system
# library is needed: every target passes -tags goolm.

# The build tree the runs live under, and what a clean up leaves of each kind.
BUILD_ROOT ?= build
KEEP_SCENARIO_RUNS ?= 5
KEEP_MUTANT_RUNS ?= 2

.PHONY: build test property acceptance acceptance-mutation clean

build:
	./scripts/build.sh

test:
	./scripts/test.sh

property:
	./scripts/property.sh

acceptance:
	./scripts/acceptance.sh

acceptance-mutation:
	./scripts/acceptance-mutation.sh

# Take away the throwaway runs a build leaves behind, keeping the newest few of
# each kind. The bridge binary lives beside them and the bridge may be running
# from it, so a rebuild after this is safe: only the runs go.
clean:
	FORGELET_KEEP_SCENARIO_RUNS=$(KEEP_SCENARIO_RUNS) FORGELET_KEEP_MUTANT_RUNS=$(KEEP_MUTANT_RUNS) ./scripts/clean.sh $(BUILD_ROOT)
