package main

import (
	"context"
	"io"
	"os"
	"path/filepath"
	"strings"
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
		skill  string
		addr   string
		out    string
		author string
	}{
		{
			name:   "bare path",
			args:   []string{"skill-review", "SKILL.md"},
			path:   "SKILL.md",
			addr:   ":8420",
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
			addr:   ":8420",
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
		{
			name:   "a target reviewed against a different skill",
			args:   []string{"skill-review", "--skill", "/skills/ideate", "report.md"},
			path:   "report.md",
			skill:  "/skills/ideate",
			addr:   ":8420",
			out:    ".skill-review",
			author: "olly",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Setenv("USER", "olly")

			var got struct{ path, skill, addr, out, author string }
			cmd := command()
			cmd.Command("serve").Action = func(_ context.Context, c *cli.Command) error {
				got.path, got.skill = c.StringArg("target"), c.String("skill")
				got.addr, got.out, got.author = c.String("addr"), c.String("out"), c.String("author")
				return nil
			}

			if err := cmd.Run(t.Context(), tt.args); err != nil {
				t.Fatalf("Run(%q): %v", tt.args, err)
			}
			if got.path != tt.path || got.skill != tt.skill || got.addr != tt.addr || got.out != tt.out || got.author != tt.author {
				t.Errorf("got %+v, want path=%q skill=%q addr=%q out=%q author=%q", got, tt.path, tt.skill, tt.addr, tt.out, tt.author)
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

// A typo'd --skill would otherwise produce a payload naming a skill nobody can apply it
// to, and say so nowhere.
func TestExplicitSkillMustBeASkill(t *testing.T) {
	dir := t.TempDir()
	target := filepath.Join(dir, "report.md")
	if err := os.WriteFile(target, []byte("# Report\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	skillDir := filepath.Join(dir, "ideate")
	if err := os.MkdirAll(skillDir, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(skillDir, "SKILL.md"), []byte("---\nname: ideate\n---\n"), 0o644); err != nil {
		t.Fatal(err)
	}

	t.Run("a skill that is not there is named in the error", func(t *testing.T) {
		cmd := command()
		err := cmd.Run(t.Context(), []string{"skill-review", "handoff", "--skill", filepath.Join(dir, "idaete"), target})
		if err == nil || !strings.Contains(err.Error(), "--skill") {
			t.Errorf("err = %v; want it to name the flag", err)
		}
	})

	t.Run("a real skill is accepted", func(t *testing.T) {
		cmd := command()
		cmd.Writer = io.Discard
		if err := cmd.Run(t.Context(), []string{
			"skill-review", "handoff", "--skill", skillDir, "--out", filepath.Join(dir, "out"), target,
		}); err != nil {
			t.Errorf("Run: %v", err)
		}
	})
}
