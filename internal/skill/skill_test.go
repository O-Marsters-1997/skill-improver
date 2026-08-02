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
		gotDir, gotMD, err := Resolve(dir)
		if err != nil || gotDir != dir || gotMD != md {
			t.Errorf("got %q, %q, %v; want %q, %q", gotDir, gotMD, err, dir, md)
		}
	})

	t.Run("a file resolves to itself and its directory", func(t *testing.T) {
		gotDir, gotMD, err := Resolve(md)
		if err != nil || gotDir != dir || gotMD != md {
			t.Errorf("got %q, %q, %v; want %q, %q", gotDir, gotMD, err, dir, md)
		}
	})

	// Mode derivation compares the two paths, so a relative one would read as output.
	t.Run("a relative path is made absolute", func(t *testing.T) {
		t.Chdir(dir)
		gotDir, gotMD, err := Resolve(FileName)
		if err != nil {
			t.Fatal(err)
		}
		if !filepath.IsAbs(gotDir) || !filepath.IsAbs(gotMD) {
			t.Errorf("got %q, %q; want absolute paths", gotDir, gotMD)
		}
	})

	t.Run("a path that does not exist is treated as a file", func(t *testing.T) {
		absent := filepath.Join(dir, "nope.md")
		gotDir, gotMD, err := Resolve(absent)
		if err != nil || gotDir != dir || gotMD != absent {
			t.Errorf("got %q, %q, %v", gotDir, gotMD, err)
		}
	})
}
