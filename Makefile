PKG := github.com/guggero/chantools

GOTEST := GO111MODULE=on go test -v

GO_BIN := ${GOPATH}/bin

GOFILES_NOVENDOR = $(shell find . -type f -name '*.go' -not -path "./vendor/*")
GOLIST := go list $(PKG)/... | grep -v '/vendor/'

LINT_BIN := $(GO_BIN)/golangci-lint
LINT_PKG := github.com/golangci/golangci-lint/cmd/golangci-lint
LINT_COMMIT := v1.18.0
LINT = $(LINT_BIN) run -v

DEPGET := cd /tmp && GO111MODULE=on go get -v
GOBUILD := GO111MODULE=on go build -v
GOINSTALL := GO111MODULE=on go install -v
GOTEST := GO111MODULE=on go test -v
XARGS := xargs -L 1

VERSION_TAG = $(shell git describe --tags)
VERSION_CHECK = @$(call print, "Building master with date version tag")

BUILD_SYSTEM = darwin-amd64 \
linux-386 \
linux-amd64 \
linux-armv6 \
linux-armv7 \
linux-arm64 \
windows-386 \
windows-amd64 \
windows-arm

# By default we will build all systems. But with the 'sys' tag, a specific
# system can be specified. This is useful to release for a subset of
# systems/architectures.
ifneq ($(sys),)
BUILD_SYSTEM = $(sys)
endif

TEST_FLAGS = -test.timeout=20m

UNIT := $(GOLIST) | $(XARGS) env $(GOTEST) $(TEST_FLAGS)
LDFLAGS := -X main.Commit=$(shell git describe --tags)
RELEASE_LDFLAGS := -s -w -buildid= $(LDFLAGS)

GREEN := "\\033[0;32m"
NC := "\\033[0m"
define print
	echo $(GREEN)$1$(NC)
endef

default: build

$(LINT_BIN):
	@$(call print, "Fetching linter")
	$(DEPGET) $(LINT_PKG)@$(LINT_COMMIT)

unit: 
	@$(call print, "Running unit tests.")
	$(UNIT)

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

fmt:
	@$(call print, "Formatting source.")
	gofmt -l -w -s $(GOFILES_NOVENDOR)

lint: $(LINT_BIN)
	@$(call print, "Linting source.")
	$(LINT)

docs: install
	@$(call print, "Rendering docs.")
	chantools doc
