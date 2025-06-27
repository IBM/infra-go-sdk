# Default target: build the project
all: build

# Build the main executable
build:
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

# Optional: Build and run the tool (for development)
run: build
	./hmc-tool