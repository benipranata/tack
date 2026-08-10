.PHONY: build test check-fixture generate-fixture

build:
	go build ./...

test:
	go test ./...

# generate-fixture regenerates the checked-in golden fixture under
# openspec/initial-idea using the current source of cmd/tack.
generate-fixture:
	go run ./cmd/tack --config openspec/initial-idea/tack.yaml

# check-fixture is the CI-freshness check for this repo's own checked-in
# golden fixture: regenerate it and fail if that changes anything, catching
# drift between the generator and the checked-in output. Consumers of tack
# run the equivalent `tack && git diff --exit-code` in their own repos.
check-fixture: generate-fixture
	git diff --exit-code -- openspec/initial-idea
