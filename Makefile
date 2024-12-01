# Targets
build: update-tools bins

bins: build-ratecat

all: update-tools generate clean bins test

clean: clean-bins clean-test-results

generate: generate-mock

.PHONY: all

# Arguments
GOOS        ?= $(shell go env GOOS)
GOARCH      ?= $(shell go env GOARCH)
GOPATH      ?= $(shell go env GOPATH)
# Disable cgo by default.
CGO_ENABLED ?= 0
LDFLAGS ?= -ldflags="-s -w"

V ?= 0
ifeq ($(V), 1)
override VERBOSE_TAG := -v
endif

# Variables
GOBIN := $(if $(shell go env GOBIN),$(shell go env GOBIN),$(GOPATH)/bin)
PATH := $(GOBIN):$(PATH)

MODULE_ROOT := $(lastword $(shell grep -e "^module " go.mod))
COLOR := "\e[1;36m%s\e[0m\n"
RED :=   "\e[1;31m%s\e[0m\n"

define NEWLINE


endef

TEST_TIMEOUT := 5m

ALL_SRC         := $(shell find . -name "*.go")
ALL_SRC         += go.mod
TEST_DIRS       := $(sort $(dir $(filter %_test.go,$(ALL_SRC))))

# Code coverage output files.
COVER_ROOT                 := ./.coverage
COVER_PROFILE         := $(COVER_ROOT)/coverprofile.out
SUMMARY_COVER_PROFILE      := $(COVER_ROOT)/summary.out

# Programs
run:
	@go run ./cmd/ratecat

# Build
build-ratecat: $(ALL_SRC)
	@printf $(COLOR) "Build sushi with CGO_ENABLED=$(CGO_ENABLED) for $(GOOS)/$(GOARCH)..."
	CGO_ENABLED=$(CGO_ENABLED) go build -tags=gc_opt -tags=poll_opt $(LDFLAGS) -o ratecat ./cmd/ratecat

# Clean
clean-bins:
	@printf $(COLOR) "Delete old binaries..."
	@rm -f ratecat

# Generate targets
generate-mock: update-mockery
	@printf $(COLOR) "Generate interface mocks..."
	@go generate ./internal/mocks/...

# Tools
update-tools: update-mockery update-linter

update-mockery:
	@printf $(COLOR) "Install/update mockery tool..."
	@go install github.com/vektra/mockery/v2@v2.26.1

update-linter:
	@printf $(COLOR) "Install/update check tools..."
	@go install github.com/golangci/golangci-lint/cmd/golangci-lint@v1.54.2

wire-inject:
	@wire ./cmd/ratecat/...

proto-gen:
	@printf $(COLOR) "Generate protobuf..."
	@./scripts/proto_gen.sh

slow-proto-gen:
	@protoc -I proto/ --go_out=proto/ --go_opt=paths=source_relative ./proto/ratecat_v1.proto

vtproto-gen:
	protoc \
	-I proto/ \
	--go_out=. \
	--go_opt=paths=source_relative \
	--plugin protoc-gen-go="$(shell which protoc-gen-go)" \
	--go-vtproto_out=. \
	--go-vtproto_opt=features=marshal+unmarshal+size+pool \
	--plugin protoc-gen-go-vtproto="$(shell which protoc-gen-go-vtproto)" \
	./proto/ratecat_api_v1.proto

# Tests
clean-test-results:
	@rm -f test.log
	@go clean -testcache

build-tests:
	@printf $(COLOR) "Build tests..."
	@go test -exec="true" -count=0 $(TEST_DIRS)

test: clean-test-results
	@printf $(COLOR) "Run tests..."
	$(foreach TEST_DIR,$(TEST_DIRS),\
		@go test $(TEST_DIR) -timeout=$(TEST_TIMEOUT) $(VERBOSE_TAG) -race | tee -a test.log \
	$(NEWLINE))
	@! grep -q "^--- FAIL" test.log

##### Coverage #####
$(COVER_ROOT):
	@mkdir -p $(COVER_ROOT)

coverage: $(COVER_ROOT)
	@printf $(COLOR) "Run unit tests with coverage..."
	@echo "mode: atomic" > $(COVER_PROFILE)
	$(foreach TEST_DIR,$(patsubst ./%/,%,$(TEST_DIRS)),\
		@mkdir -p $(COVER_ROOT)/$(TEST_DIR); \
		go test ./$(TEST_DIR) -timeout=$(TEST_TIMEOUT) -race -coverprofile=$(COVER_ROOT)/$(TEST_DIR)/coverprofile.out || exit 1; \
		grep -v -e "^mode: \w\+" $(COVER_ROOT)/$(TEST_DIR)/coverprofile.out >> $(COVER_PROFILE) || true \
	$(NEWLINE))

.PHONY: $(SUMMARY_COVER_PROFILE)
$(SUMMARY_COVER_PROFILE): $(COVER_ROOT)
	@printf $(COLOR) "Combine coverage reports to $(SUMMARY_COVER_PROFILE)..."
	@rm -f $(SUMMARY_COVER_PROFILE)
	@echo "mode: atomic" > $(SUMMARY_COVER_PROFILE)
	$(foreach COVER_PROFILE,$(wildcard $(COVER_ROOT)/*_coverprofile.out),\
		@printf "Add %s...\n" $(COVER_PROFILE); \
		grep -v -e "[Mm]ocks\?.go" -e "^mode: \w\+" $(COVER_PROFILE) >> $(SUMMARY_COVER_PROFILE) || true \
	$(NEWLINE))

coverage-report: $(SUMMARY_COVER_PROFILE)
	@printf $(COLOR) "Generate HTML report from $(SUMMARY_COVER_PROFILE) to $(SUMMARY_COVER_PROFILE).html..."
	@go tool cover -html=$(SUMMARY_COVER_PROFILE) -o $(SUMMARY_COVER_PROFILE).html

# Checks
check: lint vet

lint: update-linter
	@printf $(COLOR) "Run linter..."
	@golangci-lint run

# Misc
gomodtidy:
	@printf $(COLOR) "go mod tidy..."
	@go mod tidy
