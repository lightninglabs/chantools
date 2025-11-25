PKG := github.com/lightninglabs/chantools

GOFILES_NOVENDOR = $(shell find . -type f -name '*.go' -not -path "./vendor/*")
GOLIST := go list $(PKG)/... | grep -v '/vendor/'

GOBUILD := CGO_ENABLED=0 go build -v
GOINSTALL := CGO_ENABLED=0 go install -v
GOTEST := CGO_ENABLED=0 go test -v
XARGS := xargs -L 1

VERSION_TAG = $(shell git describe --tags)
VERSION_CHECK = @$(call print, "Building master with date version tag")

BUILD_SYSTEM = darwin-amd64 \
darwin-arm64 \
linux-386 \
linux-amd64 \
linux-armv6 \
linux-armv7 \
linux-arm64 \
windows-amd64

# By default we will build all systems. But with the 'sys' tag, a specific
# system can be specified. This is useful to release for a subset of
# systems/architectures.
ifneq ($(sys),)
BUILD_SYSTEM = $(sys)
endif

TEST_FLAGS = -test.timeout=20m

UNIT := $(GOLIST) | grep -v "/itest" | $(XARGS) env $(GOTEST) $(TEST_FLAGS)
LDFLAGS := -X main.Commit=$(shell git describe --tags)
RELEASE_LDFLAGS := -s -w -buildid= $(LDFLAGS)

GREEN := "\\033[0;32m"
NC := "\\033[0m"
define print
	echo $(GREEN)$1$(NC)
endef

default: build

unit:
	@$(call print, "Running unit tests.")
	$(UNIT)

itest: install
	@$(call print, "Running integration tests.")
	cd itest; ./itest.sh

build:
	@$(call print, "Building chantools.")
	$(GOBUILD) -ldflags "$(LDFLAGS)" ./...

install:
	@$(call print, "Installing chantools.")
	$(GOINSTALL) -ldflags "$(LDFLAGS)" ./...

release:
	@$(call print, "Creating release of chantools.")
	rm -rf chantools-v*
	./release.sh build-release "$(VERSION_TAG)" "$(BUILD_SYSTEM)" "$(RELEASE_LDFLAGS)"

docker-release:
	@$(call print, "Creating docker release of chantools.")
	./release.sh docker-release "$(VERSION_TAG)"

command-generator-build:
	@$(call print, "Building command generator.")
	cd doc/command-generator; npm install && npm run build
	mv doc/command-generator/dist/index.html doc/command-generator.html

fmt: $(GOIMPORTS_BIN)
	@$(call print, "Fixing imports.")
	go tool gosimports -w $(GOFILES_NOVENDOR)
	@$(call print, "Formatting source.")
	gofmt -l -w -s $(GOFILES_NOVENDOR)

lint:
	@$(call print, "Linting source.")
	go tool golangci-lint run -v $(LINT_WORKERS)

docs: install command-generator-build
	@$(call print, "Rendering docs.")
	chantools doc

.PHONY: unit itest build install release fmt lint docs
