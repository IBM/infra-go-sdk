# Default target: build the project
all: build

# Install dependencies
deps:
	go get golang.org/x/crypto/ssh

# Build the main executable
build: deps
	go build -o hmc-tool

# Run all tests in the project
test:
	go test ./...

# Start the godoc server for documentation
doc:
	godoc -http=:6060

# Clean up build artifacts
clean:
	rm -f hmc-tool

# Format the code
fmt:
	go fmt ./...

# Vet the code
vet:
	go vet ./...

# Run linting (requires golangci-lint)
lint:
	golangci-lint run

# Run all checks: format, vet, test
check: fmt vet test

# Optional: Build and run the tool (for development)
run: build
	./hmc-tool