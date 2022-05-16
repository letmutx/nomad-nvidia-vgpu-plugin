PLUGIN_BINARY=nomad-nvidia-vgpu-plugin
export GO111MODULE=on

default: build

.PHONY: clean
clean: ## Remove build artifacts
	rm -rf nomad-nvidia-vgpu-plugin launcher

build:
	go build -o ${PLUGIN_BINARY} ./cmd/main.go

.PHONY: eval
eval: deps build
	./launcher device ./${PLUGIN_BINARY} ./examples/config.hcl

.PHONY: fmt
fmt:
	@echo "==> Fixing source code with gofmt..."
	gofmt -s -w ./...

.PHONY: bootstrap
bootstrap: deps # install all dependencies

.PHONY: launcher
deps:  ## Install build and development dependencies
	@echo "==> Updating build dependencies..."
	go build github.com/hashicorp/nomad/plugins/shared/cmd/launcher
