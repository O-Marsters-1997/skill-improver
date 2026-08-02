default:
    @just --list

# Serve a SKILL.md for review; everything passes through, flags either side of the path:
#   just run --addr 127.0.0.1:9000 path/to/SKILL.md
#   just run config init --local
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
