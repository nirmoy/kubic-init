GO         := GO111MODULE=on GO15VENDOREXPERIMENT=1 go
GO_NOMOD   := GO111MODULE=off go
GO_VERSION := $(shell $(GO) version | sed -e 's/^[^0-9.]*\([0-9.]*\).*/\1/')
GO_BIN     := $(shell [ -n "${GOBIN}" ] && echo ${GOBIN} || (echo `echo ${GOPATH} | cut -f1 -d':'`/bin))

SOURCES_DIRS    = cmd pkg
SOURCES_DIRS_GO = ./pkg/... ./cmd/...

# go source files, ignore vendor directory
KUBIC_INIT_SRCS      = $(shell find $(SOURCES_DIRS) -type f -name '*.go' -not -path "*generated*")
KUBIC_INIT_MAIN_SRCS = $(shell find $(SOURCES_DIRS) -type f -name '*.go' -not -path "*_test.go")
KUBIC_INIT_GEN_SRCS  = $(shell grep -l -r "//go:generate" $(SOURCES_DIRS))

DEEPCOPY_FILENAME := zz_generated.deepcopy.go

# the list of all the deepcopy.go files we are going to generate
DEEPCOPY_GENERATED_FILES := $(foreach file,$(KUBIC_INIT_GEN_SRCS),$(dir $(file))$(DEEPCOPY_FILENAME))
DEEPCOPY_GENERATOR       := $(GO_BIN)/deepcopy-gen

KUBIC_INIT_EXE  = cmd/kubic-init/kubic-init
KUBIC_INIT_MAIN = cmd/kubic-init/main.go
KUBIC_INIT_CFG  = $(CURDIR)/config/kubic-init.yaml
.DEFAULT_GOAL: $(KUBIC_INIT_EXE)

# These will be provided to the target
KUBIC_INIT_VERSION    := 1.0.0
KUBIC_INIT_BUILD      := `git rev-parse HEAD 2>/dev/null`
KUBIC_INIT_BRANCH     := $(shell git rev-parse --abbrev-ref HEAD 2> /dev/null  || echo 'unknown')
KUBIC_INIT_BUILD_DATE := $(shell date +%Y%m%d-%H:%M:%S)

# Use linker flags to provide version/build settings to the target
KUBIC_INIT_LDFLAGS = -ldflags "-X=main.Version=$(KUBIC_INIT_VERSION) \
                               -X=main.Build=$(KUBIC_INIT_BUILD) \
                               -X=main.BuildDate=$(KUBIC_INIT_BUILD_DATE) \
                               -X=main.Branch=$(KUBIC_INIT_BRANCH) \
                               -X=main.GoVersion=$(GO_VERSION)"

#############################################################
# Build targets
#############################################################

all: $(KUBIC_INIT_EXE)

print-version:
	@echo "kubic-init version: $(KUBIC_INIT_VERSION)"
	@echo "kubic-init build: $(KUBIC_INIT_BUILD)"
	@echo "kubic-init branch: $(KUBIC_INIT_BRANCH)"
	@echo "kubic-init date: $(KUBIC_INIT_BUILD_DATE)"
	@echo "go: $(GO_VERSION)"

# NOTE: deepcopy-gen doesn't support go1.11's modules, so we must 'go get' it
$(DEEPCOPY_GENERATOR):
	@[ -n "${GOPATH}" ] || ( echo "FATAL: GOPATH not defined" ; exit 1 ; )
	@echo ">>> Getting deepcopy-gen (for $(DEEPCOPY_GENERATOR))"
	-@$(GO_NOMOD) get    -u k8s.io/code-generator/cmd/deepcopy-gen
	-@$(GO_NOMOD) get -d -u k8s.io/apimachinery

define _CREATE_DEEPCOPY_TARGET
$(1): $(DEEPCOPY_GENERATOR) $(shell grep -l "//go:generate" $(dir $1)/*)
	@echo ">>> Updating deepcopy files in $(dir $1)"
	@$(GO) generate -x $(dir $1)/*
endef

# Use macro to generate targets for all the DEEPCOPY_GENERATED_FILES files
$(foreach file,$(DEEPCOPY_GENERATED_FILES),$(eval $(call _CREATE_DEEPCOPY_TARGET,$(file))))

clean-generated:
	rm -f $(DEEPCOPY_GENERATED_FILES)

generate: $(DEEPCOPY_GENERATOR) $(DEEPCOPY_GENERATED_FILES)
.PHONY: generate

$(KUBIC_INIT_EXE): $(KUBIC_INIT_MAIN_SRCS) $(DEEPCOPY_GENERATED_FILES)
	@echo ">>> Building $(KUBIC_INIT_EXE)..."
	$(GO) build $(KUBIC_INIT_LDFLAGS) -o $(KUBIC_INIT_EXE) $(KUBIC_INIT_MAIN)

.PHONY: fmt
fmt: $(KUBIC_INIT_SRCS)
	@echo ">>> Reformatting code"
	@$(GO) fmt $(SOURCES_DIRS_GO)

.PHONY: simplify
simplify:
	@gofmt -s -l -w $(KUBIC_INIT_SRCS)

# NOTE: deepcopy-gen doesn't support go1.11's modules, so we must 'go get' it
$(GO_BIN)/golint:
	@echo ">>> Getting $(GO_BIN)/golint"
	@$(GO_NOMOD) get -u golang.org/x/lint/golint
	@echo ">>> $(GO_BIN)/golint successfully installed"

# once golint is fixed add here option to golint: set_exit_status 
#  see https://github.com/kubic-project/kubic-init/issues/69
.PHONY: check
check: $(GO_BIN)/golint
	@test -z $(shell gofmt -l $(KUBIC_INIT_MAIN) | tee /dev/stderr) || echo "[WARN] Fix formatting issues with 'make fmt'"
	@for d in $$($(GO) list ./... | grep -v /vendor/); do $(GO_BIN)/golint $${d}; done
	@$(GO) tool vet ${KUBIC_INIT_SRCS}
	terraform fmt
lint: check

.PHONY: test
test:
	@$(GO) test -v ./pkg/... ./cmd/... -coverprofile cover.out

.PHONY: check
clean:
	rm -f $(KUBIC_INIT_EXE)
	rm -f config/rbac/*.yaml config/crds/*.yaml

.PHONY: coverage
coverage: 
	$(GO_NOMOD) tool cover -html=cover.out

include build/make/*.mk
