GO ?= go
PUBLIC_MODULE ?= github.com/portpowered/go-google-nest-sdm
PUBLIC_PACKAGES ?= pkg/sdm,pkg/dependencies/httptransport,pkg/dependencies/media,pkg/dependencymodels
export GOWORK := off

.DEFAULT_GOAL := check
.PHONY: check build test test-race test-cover test-integration vet lint fmt test-contracts routegate generate-api check-generated check-modules check-format api-compatibility

check: lint build test-contracts routegate test vet test-cover check-format check-modules check-generated

build:
	$(GO) run ./tools/verify -mode build

test:
	$(GO) run ./tools/verify -mode test

test-race: test

vet:
	$(GO) run ./tools/verify -mode vet

lint:
	$(GO) run ./tools/verify -mode lint

fmt:
	$(GO) run ./tools/verify -mode fmt

test-contracts:
	npm ci --prefix tools/generate --ignore-scripts
	node tools/generate/validate.mjs
	$(GO) run ./tools/contractcheck

routegate:
	$(GO) run ./tools/routegate

test-cover:
	$(GO) test -race -coverpkg=./pkg/... -coverprofile=coverage-unit.out ./pkg/...
	$(GO) test -race -coverpkg=./pkg/... -coverprofile=coverage-replay.out ./tests/replay/...
	$(GO) test -race -coverpkg=./pkg/... -coverprofile=coverage-combined.out ./pkg/... ./tests/replay/...
	$(GO) run ./tools/coverage -profile coverage-unit.out -min 0 -filtered-profile coverage-unit-filtered.out
	$(GO) run ./tools/coverage -profile coverage-replay.out -min 0 -filtered-profile coverage-replay-filtered.out
	$(GO) run ./tools/coverage -profile coverage-combined.out -min 80 -filtered-profile coverage-combined-filtered.out

test-integration:
	$(GO) test -tags integration -race -coverpkg=./pkg/... -coverprofile=coverage-integration.out ./tests/integration/... -timeout=5m
	$(GO) run ./tools/coverage -profile coverage-integration.out -min 0 -filtered-profile coverage-integration-filtered.out

generate-api:
	npm ci --prefix tools/generate --ignore-scripts
	$(GO) run ./tools/generate

check-generated: generate-api
	$(GO) run ./tools/verify -mode generated

check-modules:
	$(GO) run ./tools/verify -mode modules

check-format:
	$(GO) run ./tools/verify -mode format-check

api-compatibility:
	$(GO) run ./tools/compatibility -policy report -base previous-release -module "$(PUBLIC_MODULE)" -packages "$(PUBLIC_PACKAGES)"
