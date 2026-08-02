package skill

import "testing"

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
