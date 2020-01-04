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

TEST_FLAGS = -test.timeout=20m

UNIT := $(GOLIST) | $(XARGS) env $(GOTEST) $(TEST_FLAGS)

default: build

$(LINT_BIN):
	@$(call print, "Fetching linter")
	$(DEPGET) $(LINT_PKG)@$(LINT_COMMIT)

unit: 
	@$(call print, "Running unit tests.")
	$(UNIT)

build:
	@$(call print, "Building chantools.")
	$(GOBUILD) ./...

install:
	@$(call print, "Installing chantools.")
	$(GOINSTALL) ./...

fmt:
	@$(call print, "Formatting source.")
	gofmt -l -w -s $(GOFILES_NOVENDOR)

lint: $(LINT_BIN)
	@$(call print, "Linting source.")
	$(LINT)