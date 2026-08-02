default:
    @just --list

# The frontend (web/) is built by Vite/bun straight into internal/server/web, which Go
# embeds — see internal/server/server.go. That directory is gitignored except
# not-built.html, so this has to run before anything that builds or runs the binary.
web:
    @command -v bun >/dev/null || { echo "bun is required to build the frontend — https://bun.sh" >&2; exit 1; }
    cd web && bun install --frozen-lockfile && bun run build

# Serve a SKILL.md for review; everything passes through, flags either side of the path:
#   just run --addr 127.0.0.1:9000 path/to/SKILL.md
#   just run config init --local
run *ARGS='testdata/example-skill': web
    go run ./cmd/skill-review {{ARGS}}

build: web
    go build -o bin/skill-review ./cmd/skill-review

install: web
    go install ./cmd/skill-review

test *ARGS:
    go test ./... {{ARGS}}

# The property the tool rests on: a span's text is exactly the source bytes at its offset
fuzz TIME='30s':
    go test ./internal/render -run '^$' -fuzz FuzzOffsets -fuzztime {{TIME}}
    go test ./internal/render -run '^$' -fuzz FuzzHTMLOffsets -fuzztime {{TIME}}

fmt:
    gofmt -w cmd internal

vet:
    go vet ./...

check: vet test
    @unformatted=$(gofmt -l cmd internal); \
    if [ -n "$unformatted" ]; then echo "unformatted:"; echo "$unformatted"; exit 1; fi

clean:
    rm -rf bin
    rm -f internal/server/web/index.html internal/server/web/app.js internal/server/web/app.css internal/server/web/favicon.svg
