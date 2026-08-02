package main

import (
	"bytes"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/O-Marsters-1997/improve-skills/internal/config"
)

// run drives the real command in a scratch working directory, so --local writes there and
// the user config path cannot be reached by accident.
func run(t *testing.T, args ...string) (string, error) {
	t.Helper()
	t.Chdir(t.TempDir())

	var out bytes.Buffer
	cmd := command()
	cmd.Writer = &out
	cmd.ErrWriter = &out

	err := cmd.Run(t.Context(), append([]string{"skill-review"}, args...))
	return out.String(), err
}

func updaterSkill(t *testing.T, name string) string {
	t.Helper()
	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, "SKILL.md"), []byte("---\nname: "+name+"\n---\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	return dir
}

func TestConfigInit(t *testing.T) {
	t.Run("writes a config that loads back as the defaults", func(t *testing.T) {
		out, err := run(t, "config", "init", "--local")
		if err != nil {
			t.Fatal(err)
		}

		cfg, err := config.Load(config.LocalName)
		if err != nil {
			t.Fatalf("the file it just wrote does not load: %v", err)
		}
		if len(cfg.Fields) != 3 || cfg.Fields[0].Name != "priority" {
			t.Errorf("fields = %+v; want the defaults", cfg.Fields)
		}
		for _, want := range []string{"wrote", config.LocalName, "updater   none", "priority", "category", "cause"} {
			if !strings.Contains(out, want) {
				t.Errorf("output does not mention %q:\n%s", want, out)
			}
		}
	})

	t.Run("records a configured updater and names it back", func(t *testing.T) {
		dir := updaterSkill(t, "my-updater")

		out, err := run(t, "config", "init", "--local", "--updater", dir)
		if err != nil {
			t.Fatal(err)
		}

		cfg, err := config.Load(config.LocalName)
		if err != nil {
			t.Fatal(err)
		}
		if cfg.Updater.Skill != dir || cfg.Updater.Name != "my-updater" {
			t.Errorf("updater = %+v; want skill=%q name=%q", cfg.Updater, dir, "my-updater")
		}
		if !strings.Contains(out, "my-updater") {
			t.Errorf("output does not name the updater:\n%s", out)
		}
	})

	t.Run("refuses to overwrite without --force", func(t *testing.T) {
		t.Chdir(t.TempDir())
		if err := os.WriteFile(config.LocalName, []byte("# mine\n"), 0o644); err != nil {
			t.Fatal(err)
		}

		cmd := command()
		cmd.Writer, cmd.ErrWriter = new(bytes.Buffer), new(bytes.Buffer)
		err := cmd.Run(t.Context(), []string{"skill-review", "config", "init", "--local"})

		if err == nil || !strings.Contains(err.Error(), "--force") {
			t.Fatalf("err = %v; want a refusal mentioning --force", err)
		}
		if body, _ := os.ReadFile(config.LocalName); string(body) != "# mine\n" {
			t.Errorf("the existing file was overwritten: %q", body)
		}
	})

	t.Run("--force overwrites", func(t *testing.T) {
		t.Chdir(t.TempDir())
		if err := os.WriteFile(config.LocalName, []byte("# mine\n"), 0o644); err != nil {
			t.Fatal(err)
		}

		cmd := command()
		cmd.Writer, cmd.ErrWriter = new(bytes.Buffer), new(bytes.Buffer)
		if err := cmd.Run(t.Context(), []string{"skill-review", "config", "init", "--local", "--force"}); err != nil {
			t.Fatal(err)
		}
		if body, _ := os.ReadFile(config.LocalName); strings.Contains(string(body), "# mine") {
			t.Error("the file was not replaced")
		}
	})
}

// The point of validating on the flag is that nothing is written when the path is wrong.
func TestConfigInitRejectsBadUpdater(t *testing.T) {
	tests := []struct {
		name    string
		updater string
		want    string
	}{
		{name: "relative", updater: "skills/updater", want: "is not an absolute path"},
		{name: "missing", updater: "/nowhere/at/all", want: "cannot read"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, err := run(t, "config", "init", "--local", "--updater", tt.updater)
			if err == nil || !strings.Contains(err.Error(), tt.want) {
				t.Fatalf("err = %v; want it to contain %q", err, tt.want)
			}
			if _, err := os.Stat(config.LocalName); err == nil {
				t.Error("a config file was written despite the bad updater")
			}
		})
	}
}
