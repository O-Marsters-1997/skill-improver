package main

import (
	"context"
	"testing"

	"github.com/urfave/cli/v3"
)

// The bare invocation is the documented one, so DefaultCommand routing is the thing most
// worth pinning: every spelling below has to reach serve with the same values.
func TestServeDispatch(t *testing.T) {
	tests := []struct {
		name   string
		args   []string
		path   string
		addr   string
		out    string
		author string
	}{
		{
			name:   "bare path",
			args:   []string{"skill-review", "SKILL.md"},
			path:   "SKILL.md",
			addr:   "127.0.0.1:8420",
			out:    ".skill-review",
			author: "olly",
		},
		{
			name:   "flags before the path",
			args:   []string{"skill-review", "--addr", ":9000", "--out", "elsewhere", "--author", "sam", "SKILL.md"},
			path:   "SKILL.md",
			addr:   ":9000",
			out:    "elsewhere",
			author: "sam",
		},
		{
			name:   "explicit serve",
			args:   []string{"skill-review", "serve", "SKILL.md"},
			path:   "SKILL.md",
			addr:   "127.0.0.1:8420",
			out:    ".skill-review",
			author: "olly",
		},
		{
			name:   "flags after serve",
			args:   []string{"skill-review", "serve", "--addr", ":9000", "SKILL.md"},
			path:   "SKILL.md",
			addr:   ":9000",
			out:    ".skill-review",
			author: "olly",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Setenv("USER", "olly")

			var got struct{ path, addr, out, author string }
			cmd := command()
			cmd.Command("serve").Action = func(_ context.Context, c *cli.Command) error {
				got.path = c.StringArg("skill")
				got.addr, got.out, got.author = c.String("addr"), c.String("out"), c.String("author")
				return nil
			}

			if err := cmd.Run(t.Context(), tt.args); err != nil {
				t.Fatalf("Run(%q): %v", tt.args, err)
			}
			if got.path != tt.path || got.addr != tt.addr || got.out != tt.out || got.author != tt.author {
				t.Errorf("got %+v, want path=%q addr=%q out=%q author=%q", got, tt.path, tt.addr, tt.out, tt.author)
			}
		})
	}
}

// The logged URL is the only address the reviewer ever sees, so a wildcard listener has to
// be reported as something a browser can actually open.
func TestBrowsableURL(t *testing.T) {
	tests := []struct{ addr, want string }{
		{"127.0.0.1:8420", "http://127.0.0.1:8420"},
		{":9000", "http://localhost:9000"},
		{"0.0.0.0:8420", "http://localhost:8420"},
		{"[::]:8420", "http://localhost:8420"},
		{"[::1]:8420", "http://[::1]:8420"},
		{"example.test:80", "http://example.test:80"},
		{"nonsense", "http://nonsense"},
	}

	for _, tt := range tests {
		t.Run(tt.addr, func(t *testing.T) {
			if got := browsableURL(tt.addr); got != tt.want {
				t.Errorf("browsableURL(%q) = %q, want %q", tt.addr, got, tt.want)
			}
		})
	}
}

func TestMissingPathExitsTwo(t *testing.T) {
	cmd := command()
	// Without a handler, cli calls os.Exit on an ExitCoder and takes the test with it.
	cmd.ExitErrHandler = func(context.Context, *cli.Command, error) {}

	err := cmd.Run(t.Context(), []string{"skill-review"})

	exit, ok := err.(cli.ExitCoder)
	if !ok {
		t.Fatalf("err = %v, want a cli.ExitCoder", err)
	}
	if exit.ExitCode() != 2 {
		t.Errorf("exit code = %d, want 2", exit.ExitCode())
	}
}
