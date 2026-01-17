# Build the library
build:
    go build -v ./...

# Run all tests
test:
    go test -v -race -count=1 ./...

# Run benchmarks
bench:
    go test -bench=. -benchmem -run=^$ ./...

# Run linters
lint:
    golangci-lint run

# Run linters and fix issues
lint-fix:
    golangci-lint run --fix

# Format code using treefmt
fmt:
    treefmt . --allow-missing-formatter

# Check if code is formatted
fmt-check:
    treefmt --allow-missing-formatter --fail-on-change

# Generate coverage report
cover:
    go test -coverprofile=coverage.txt -covermode=atomic ./...
    go tool cover -html=coverage.txt -o coverage.html

# Clean build artifacts
clean:
    rm -f coverage.txt coverage.html

# Run all checks (test, lint, fmt-check)
check: test lint fmt-check

# Cross-compile for AMD64
build-amd64:
    GOOS=linux GOARCH=amd64 go build -v ./...

# Cross-compile for ARM64
build-arm64:
    GOOS=linux GOARCH=arm64 go build -v ./...

# Cross-compile for Darwin ARM64 (macOS Apple Silicon)
build-darwin-arm64:
    GOOS=darwin GOARCH=arm64 go build -v ./...

# Cross-compile for Windows AMD64
build-windows:
    GOOS=windows GOARCH=amd64 go build -v ./...

# Run tests on ARM64 using QEMU (requires qemu-user-static)
test-arm64:
    #!/usr/bin/env bash
    if ! command -v qemu-aarch64-static &> /dev/null; then
        echo "Error: qemu-aarch64-static not found"
        echo "Install with: sudo apt-get install qemu-user-static binfmt-support"
        exit 1
    fi
    GOOS=linux GOARCH=arm64 go test -v -count=1 ./...

# Run benchmarks on ARM64 using QEMU (NOTE: performance not representative, correctness only)
bench-arm64:
    #!/usr/bin/env bash
    if ! command -v qemu-aarch64-static &> /dev/null; then
        echo "Error: qemu-aarch64-static not found"
        echo "Install with: sudo apt-get install qemu-user-static binfmt-support"
        exit 1
    fi
    @echo "NOTE: QEMU benchmarks are for correctness validation only, not performance measurement"
    GOOS=linux GOARCH=arm64 go test -bench=. -benchmem -run=^$ ./...

# Build for all platforms
build-all: build build-amd64 build-arm64 build-darwin-arm64 build-windows
    @echo "Built for all platforms: amd64, arm64, darwin-arm64, windows"

# Test on both amd64 and arm64
test-all: test test-arm64
    @echo "Tests passed on both architectures"

# Run all checks on both architectures
check-all: check test-arm64
    @echo "All checks passed on amd64 and arm64"

# Run calibration example
example-calibrate:
    cd examples/calibrate && go run main.go

# Run drift example
example-drift:
    cd examples/drift && go run main.go

# Profile CPU usage
profile-cpu:
    go test -cpuprofile=cpu.prof -bench=. ./...
    go tool pprof -http=:8080 cpu.prof

# Profile memory usage
profile-mem:
    go test -memprofile=mem.prof -bench=. ./...
    go tool pprof -http=:8080 mem.prof

# Default target
default: build
