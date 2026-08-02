package skill

import (
	"os"
	"path/filepath"
	"testing"
)

func TestName(t *testing.T) {
	tests := []struct {
		name string
		src  string
		want string
	}{
		{
			name: "frontmatter name",
			src:  "---\nname: skill-updater\ndescription: does things\n---\n\n# Heading\n",
			want: "skill-updater",
		},
		{
			name: "quoted value",
			src:  "---\nname: \"quoted-name\"\n---\n",
			want: "quoted-name",
		},
		{
			name: "no frontmatter",
			src:  "# Just a heading\n",
			want: "",
		},
		{
			name: "name outside the frontmatter block is ignored",
			src:  "---\ndescription: x\n---\n\nname: not-this\n",
			want: "",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := Name([]byte(tt.src)); got != tt.want {
				t.Errorf("got %q; want %q", got, tt.want)
			}
		})
	}
}

func TestResolve(t *testing.T) {
	dir := t.TempDir()
	md := filepath.Join(dir, FileName)
	if err := os.WriteFile(md, []byte("---\nname: x\n---\n"), 0o644); err != nil {
		t.Fatal(err)
	}

	t.Run("a directory resolves to the SKILL.md inside it", func(t *testing.T) {
		got, err := Resolve(dir)
		if err != nil || got != md {
			t.Errorf("got %q, %v; want %q", got, err, md)
		}
	})

	t.Run("a file resolves to itself", func(t *testing.T) {
		got, err := Resolve(md)
		if err != nil || got != md {
			t.Errorf("got %q, %v; want %q", got, err, md)
		}
	})

	// Mode derivation compares two paths, so a relative one would read as output.
	t.Run("a relative path is made absolute", func(t *testing.T) {
		t.Chdir(dir)
		got, err := Resolve(FileName)
		if err != nil {
			t.Fatal(err)
		}
		if !filepath.IsAbs(got) {
			t.Errorf("got %q; want an absolute path", got)
		}
	})

	t.Run("a path that does not exist is treated as a file", func(t *testing.T) {
		absent := filepath.Join(dir, "nope.md")
		got, err := Resolve(absent)
		if err != nil || got != absent {
			t.Errorf("got %q, %v", got, err)
		}
	})
}
