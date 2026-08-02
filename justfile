default:
    @just --list

# Serve a SKILL.md for review; flags pass through: just run --addr :9000 path/to/SKILL.md
run *ARGS='testdata/example-SKILL.md':
    go run ./cmd/skill-review {{ARGS}}

build:
    go build -o bin/skill-review ./cmd/skill-review

install:
    go install ./cmd/skill-review

test *ARGS:
    go test ./... {{ARGS}}

# The property the tool rests on: a span's text is exactly the source bytes at its offset
fuzz TIME='30s':
    go test ./internal/render -run '^$' -fuzz FuzzOffsets -fuzztime {{TIME}}

fmt:
    gofmt -w cmd internal

vet:
    go vet ./...

check: vet test
    @unformatted=$(gofmt -l cmd internal); \
    if [ -n "$unformatted" ]; then echo "unformatted:"; echo "$unformatted"; exit 1; fi

clean:
    rm -rf bin
