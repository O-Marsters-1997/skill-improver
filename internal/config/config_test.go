package config

import (
	"errors"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"
)

func write(t *testing.T, body string) string {
	t.Helper()
	path := filepath.Join(t.TempDir(), LocalName)
	if err := os.WriteFile(path, []byte(body), 0o644); err != nil {
		t.Fatal(err)
	}
	return path
}

// The template is the only documentation most people will read, so it has to stay in
// step with the defaults it claims to be showing.
func TestTemplateRoundTripsToDefault(t *testing.T) {
	got, err := Load(write(t, Template("")))
	if err != nil {
		t.Fatalf("Load(Template()): %v", err)
	}
	if !reflect.DeepEqual(got, Default()) {
		t.Errorf("Load(Template()) = %+v\nwant %+v", got, Default())
	}
}

func TestLoad(t *testing.T) {
	t.Run("a custom schema replaces the defaults entirely", func(t *testing.T) {
		got, err := Load(write(t, `
[[field]]
name    = "severity"
label   = "How bad"
values  = ["blocker", "nit"]
default = "nit"
`))
		if err != nil {
			t.Fatal(err)
		}

		want := []Field{{Name: "severity", Label: "How bad", Values: []string{"blocker", "nit"}, Default: "nit"}}
		if !reflect.DeepEqual(got.Fields, want) {
			t.Errorf("fields = %+v; want %+v", got.Fields, want)
		}
	})

	t.Run("a file with no fields keeps the defaults", func(t *testing.T) {
		got, err := Load(write(t, "# nothing here\n"))
		if err != nil {
			t.Fatal(err)
		}
		if !reflect.DeepEqual(got.Fields, Default().Fields) {
			t.Errorf("fields = %+v; want the defaults", got.Fields)
		}
	})

	t.Run("a missing label falls back to the name", func(t *testing.T) {
		got, err := Load(write(t, "[[field]]\nname = \"area\"\nvalues = [\"a\"]\ndefault = \"a\"\n"))
		if err != nil {
			t.Fatal(err)
		}
		if got.Fields[0].Label != "area" {
			t.Errorf("label = %q; want %q", got.Fields[0].Label, "area")
		}
	})

	t.Run("a missing file is not a failure", func(t *testing.T) {
		got, err := Load(filepath.Join(t.TempDir(), "absent.toml"))
		if !errors.Is(err, ErrNoConfig) {
			t.Fatalf("err = %v; want ErrNoConfig", err)
		}
		if !reflect.DeepEqual(got, Default()) {
			t.Errorf("got %+v; want the defaults", got)
		}
	})
}

func TestLoadRejects(t *testing.T) {
	tests := []struct {
		name string
		body string
		want string
	}{
		{
			name: "broken toml points at the line",
			body: "[[field]\nname = \"x\"\n",
			want: "At line 2, column 8",
		},
		{
			name: "unknown key",
			body: "[[field]]\nname = \"x\"\nvalues = [\"a\"]\ndefault = \"a\"\ncatgory = \"typo\"\n",
			want: `unknown key "field.catgory"`,
		},
		{
			name: "no name",
			body: "[[field]]\nvalues = [\"a\"]\ndefault = \"a\"\n",
			want: "field 1: needs a name",
		},
		{
			name: "name is not an identifier",
			body: "[[field]]\nname = \"My Field\"\nvalues = [\"a\"]\ndefault = \"a\"\n",
			want: "name must be lower-case",
		},
		{
			name: "a name the thread already uses",
			body: "[[field]]\nname = \"status\"\nvalues = [\"a\"]\ndefault = \"a\"\n",
			want: "status is reserved by the thread itself",
		},
		{
			name: "duplicate name",
			body: "[[field]]\nname = \"a\"\nvalues = [\"x\"]\ndefault = \"x\"\n\n[[field]]\nname = \"a\"\nvalues = [\"y\"]\ndefault = \"y\"\n",
			want: `field "a": declared twice`,
		},
		{
			name: "no values",
			body: "[[field]]\nname = \"a\"\nvalues = []\ndefault = \"x\"\n",
			want: "needs at least one value",
		},
		{
			name: "no default",
			body: "[[field]]\nname = \"a\"\nvalues = [\"x\", \"y\"]\n",
			want: "needs a default, one of x, y",
		},
		{
			name: "default outside the values",
			body: "[[field]]\nname = \"a\"\nvalues = [\"x\"]\ndefault = \"b\"\n",
			want: `default "b" is not one of x`,
		},
		{
			name: "relative updater path",
			body: "[updater]\nskill = \"skills/updater\"\n",
			want: "is not an absolute path",
		},
		{
			name: "updater that is not there",
			body: "[updater]\nskill = \"/nowhere/at/all\"\n",
			want: "cannot read /nowhere/at/all",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, err := Load(write(t, tt.body))
			if err == nil {
				t.Fatal("want an error, got none")
			}
			if !strings.Contains(err.Error(), tt.want) {
				t.Errorf("err = %q\nwant it to contain %q", err, tt.want)
			}
		})
	}
}

func TestResolve(t *testing.T) {
	t.Run("the project file wins over the user one", func(t *testing.T) {
		t.Chdir(t.TempDir())
		if err := os.WriteFile(LocalName, []byte("[[field]]\nname=\"local\"\nvalues=[\"a\"]\ndefault=\"a\"\n"), 0o644); err != nil {
			t.Fatal(err)
		}

		cfg, path, err := Resolve("")
		if err != nil {
			t.Fatal(err)
		}
		if path != LocalName || cfg.Fields[0].Name != "local" {
			t.Errorf("resolved %q -> %+v", path, cfg.Fields)
		}
	})

	t.Run("nothing found means the defaults, not an error", func(t *testing.T) {
		t.Setenv("XDG_CONFIG_HOME", t.TempDir())
		t.Chdir(t.TempDir())

		cfg, path, err := Resolve("")
		if err != nil || path != "" {
			t.Fatalf("path = %q, err = %v; want no file and no error", path, err)
		}
		if !reflect.DeepEqual(cfg, Default()) {
			t.Errorf("got %+v; want the defaults", cfg)
		}
	})

	// An explicit --config is a claim the file exists; falling back would ignore it.
	t.Run("an explicit config that is not there is an error", func(t *testing.T) {
		_, _, err := Resolve(filepath.Join(t.TempDir(), "absent.toml"))
		if err == nil || !strings.Contains(err.Error(), "--config") {
			t.Errorf("err = %v; want it to name the flag", err)
		}
	})
}

func TestUpdaterName(t *testing.T) {
	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, "SKILL.md"), []byte("---\nname: my-updater\n---\n"), 0o644); err != nil {
		t.Fatal(err)
	}

	t.Run("a directory resolves through its SKILL.md", func(t *testing.T) {
		got, err := UpdaterName(dir)
		if err != nil || got != "my-updater" {
			t.Errorf("got %q, %v; want %q", got, err, "my-updater")
		}
	})

	t.Run("a SKILL.md resolves directly", func(t *testing.T) {
		got, err := UpdaterName(filepath.Join(dir, "SKILL.md"))
		if err != nil || got != "my-updater" {
			t.Errorf("got %q, %v; want %q", got, err, "my-updater")
		}
	})

	t.Run("a skill with no name is rejected", func(t *testing.T) {
		bare := filepath.Join(t.TempDir(), "SKILL.md")
		if err := os.WriteFile(bare, []byte("# no frontmatter\n"), 0o644); err != nil {
			t.Fatal(err)
		}
		if _, err := UpdaterName(bare); err == nil || !strings.Contains(err.Error(), "no name:") {
			t.Errorf("err = %v; want a complaint about the frontmatter", err)
		}
	})

	t.Run("a configured updater is resolved to its name", func(t *testing.T) {
		got, err := Load(write(t, "[updater]\nskill = "+quote(dir)+"\n"))
		if err != nil {
			t.Fatal(err)
		}
		if got.Updater.Name != "my-updater" {
			t.Errorf("updater name = %q; want %q", got.Updater.Name, "my-updater")
		}
	})
}

func quote(s string) string { return `"` + s + `"` }
