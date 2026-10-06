# pholio task runner. Run `just` to list recipes.

lint_version := "2.14.0"

# List recipes
default:
    @just --list

# Build the pholio binary into ./bin
build:
    go build -o bin/pholio ./cmd/pholio

# Run pholio, e.g. `just run ~/notes`
run *args:
    go run ./cmd/pholio {{args}}

# Run the tests (pass a package or flags, e.g. `just test ./internal/engine -run Undo`)
test *args="./...":
    go test {{args}}

# Run the tests with the race detector, as CI does
race:
    go test -race ./...

# Regenerate golden View() snapshots
golden:
    go test ./internal/app ./internal/editor -update

# Run the benchmarks (pass a pattern to narrow, e.g. `just bench Frame20k`)
bench pattern=".":
    go test -run '^$' -bench '{{pattern}}' -benchmem ./...

# Run go vet
vet:
    go vet ./...

# Lint with the golangci-lint version CI pins
lint:
    mise exec golangci-lint@{{lint_version}} -- golangci-lint run

# Format the code
fmt:
    gofmt -w .

# Everything CI runs: vet, race tests, lint
ci: vet race lint

# Remove build output
clean:
    rm -rf bin
