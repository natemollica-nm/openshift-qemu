# Variables
BINARY_NAME := openshift-qemu
OUTPUT_DIR := bin
CGO_LDFLAGS := $(shell pkg-config --libs libvirt libvirt-qemu)
GOARCH_X86 := amd64
GOARCH_ARM := arm64
GOOS := linux

# Default target: build for both x86_64 and arm64
.PHONY: all
all: build_x86_64 build_arm64

# Ensure the output directory exists
.PHONY: prepare
prepare:
	@mkdir -p $(OUTPUT_DIR)

# Build for x86_64
.PHONY: build_x86_64
build_x86_64: prepare
	@echo "Building for x86_64..."
	CGO_LDFLAGS="$(CGO_LDFLAGS)" GOOS=$(GOOS) GOARCH=$(GOARCH_X86) go build -o $(OUTPUT_DIR)/$(BINARY_NAME)-x86_64 .

# Build for arm64
.PHONY: build_arm64
build_arm64: prepare
	@echo "Building for arm64..."
	CGO_LDFLAGS="$(CGO_LDFLAGS)" GOOS=$(GOOS) GOARCH=$(GOARCH_ARM) go build -o $(OUTPUT_DIR)/$(BINARY_NAME)-arm64 .

# Clean up binaries
.PHONY: clean
clean:
	@echo "Cleaning up binaries..."
	@rm -rf $(OUTPUT_DIR)

# Run the binary (useful for testing)
.PHONY: run
run: build_x86_64
	@./$(OUTPUT_DIR)/$(BINARY_NAME)-x86_64
