.PHONY: proto proto-gen proto-validate clean

# Proto directories
PROTO_DIR := ./proto
GO_OUT := ./v1

# Protobuf compiler settings
PROTOC := protoc
PROTOC_GEN_GO := protoc-gen-go
PROTOC_GEN_GO_GRPC := protoc-gen-go-grpc

# Proto files
PROTO_FILES := $(wildcard $(PROTO_DIR)/*.proto)

# Generated Go files
GENERATED_FILES := $(patsubst $(PROTO_DIR)/%.proto,$(GO_OUT)/%.pb.go,$(PROTO_FILES))
GRPC_FILES := $(patsubst $(PROTO_DIR)/%.proto,$(GO_OUT)/%_grpc.pb.go,$(PROTO_FILES))

# Default target
proto: proto-gen

# Generate protobuf Go code
proto-gen: $(GENERATED_FILES) $(GRPC_FILES)

$(GO_OUT)/%.pb.go: $(PROTO_DIR)/%.proto
	@mkdir -p $(GO_OUT)
	$(PROTOC) --go_out=. --go_opt=paths=source_relative \
		--go-grpc_out=. --go-grpc_opt=paths=source_relative \
		$<

# Validate proto files (syntax check)
proto-validate:
	@for f in $(PROTO_FILES); do \
		$(PROTOC) --proto_path=$(PROTO_DIR) --cpp_out=/dev/null $$f || exit 1; \
	done
	@echo "All proto files are valid"

# Clean generated files
clean:
	rm -rf $(GO_OUT)

help:
	@echo "Spotify package Makefile targets:"
	@echo "  proto           - Generate proto files (default)"
	@echo "  proto-gen       - Generate protobuf Go code"
	@echo "  proto-validate  - Validate proto files syntax"
	@echo "  clean           - Remove generated files"
	@echo "  help            - Show this help message"
