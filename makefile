SHELL = /bin/sh
BUILD_INFO_IMPORT_PATH=github.com/will7200/cctver/internal/version
VERSION=$(shell git describe --always --match "v[0-9]*" HEAD)
BUILD_DATE=`date +%FT%T%z`
BUILD_INFO=-ldflags "-X $(BUILD_INFO_IMPORT_PATH).Version=$(VERSION) -X $(BUILD_INFO_IMPORT_PATH).Date=$(BUILD_DATE)"
# SRC_ROOT is the top of the source tree.
SRC_ROOT := $(shell git rev-parse --show-toplevel)
TOOLS_MOD_DIR   := $(SRC_ROOT)/internal/tools
TOOLS_MOD_REGEX := "\s+_\s+\".*\""
TOOLS_PKG_NAMES := $(shell grep -E $(TOOLS_MOD_REGEX) < $(TOOLS_MOD_DIR)/tools.go | tr -d " _\"" | grep -vE '/v[0-9]+$$')
TOOLS_BIN_DIR   := $(SRC_ROOT)/.tools
TOOLS_BIN_NAMES  := $(addprefix $(TOOLS_BIN_DIR)/, $(notdir $(TOOLS_PKG_NAMES)))

# installs all the tools that we use to lint our code
.PHONY: install-tools
install-tools: $(TOOLS_BIN_NAMES)

$(TOOLS_BIN_DIR):
	mkdir -p $@

$(TOOLS_BIN_NAMES): $(TOOLS_BIN_DIR) $(TOOLS_MOD_DIR)/go.mod
	cd $(TOOLS_MOD_DIR) && go build -o $@ -trimpath $(filter %/$(notdir $@),$(TOOLS_PKG_NAMES))

# Build the cctver executable.
.PHONY: cctver
cctver:
	CGO_ENABLED=1 go build -trimpath -o ./bin/cctver \
		$(BUILD_INFO) ./cmd/cctver