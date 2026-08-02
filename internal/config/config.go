// Package config describes the shape of a review: what the thread cards ask for, what
// each suggestion in the handoff payload carries, and which skill applies them.
//
// A missing config file is not an error. Default() is the shape this tool shipped with,
// so an unconfigured machine behaves exactly as it did before there was a config at all.
package config

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"slices"
	"strings"

	"github.com/BurntSushi/toml"

	"github.com/O-Marsters-1997/improve-skills/internal/skill"
)

// Name is the JSON key on the thread and on the suggestion; Label is what the sidebar
// shows above the control.
type Field struct {
	Name    string   `toml:"name" json:"name"`
	Label   string   `toml:"label" json:"label"`
	Values  []string `toml:"values" json:"values"`
	Default string   `toml:"default" json:"default"`
}

// Skill is an absolute path to a skill directory or a SKILL.md. Name is read from its
// frontmatter at load, and is what the handoff prompt and the Submit button say.
type Updater struct {
	Skill string `toml:"skill"`
	Name  string `toml:"-"`
}

type Config struct {
	Fields  []Field `toml:"field"`
	Updater Updater `toml:"updater"`
}

func Default() *Config {
	return &Config{
		Fields: []Field{
			{
				Name:    "priority",
				Label:   "Priority",
				Values:  []string{"high", "medium", "low"},
				Default: "medium",
			},
			{
				Name:    "category",
				Label:   "Category",
				Values:  []string{"instructions", "tools", "examples", "error_handling", "structure", "references"},
				Default: "instructions",
			},
			// So a one-off model fluke can be marked as such instead of being baked
			// into a permanent instruction edit.
			{
				Name:    "cause",
				Label:   "Cause",
				Values:  []string{"instructions", "execution"},
				Default: "instructions",
			},
		},
	}
}

// Suggestions are ordered by the first field, in the order its values are listed, so the
// first field is the one worth making a ranking.
func (c *Config) SortField() (Field, bool) {
	if len(c.Fields) == 0 {
		return Field{}, false
	}
	return c.Fields[0], true
}

func (c *Config) Field(name string) (Field, bool) {
	i := slices.IndexFunc(c.Fields, func(f Field) bool { return f.Name == name })
	if i < 0 {
		return Field{}, false
	}
	return c.Fields[i], true
}

// ErrNoConfig reports that nothing was found to load, which callers treat as "use the
// defaults" rather than as a failure.
var ErrNoConfig = errors.New("config: no config file")

const (
	// LocalName sits next to the work, the same reasoning as the default out directory.
	LocalName = "skill-review.toml"
	userDir   = "skill-review"
	userName  = "config.toml"
)

// Resolve walks --config, then the project file, then the user file, and falls back to
// Default. An explicit --config that does not exist is an error; the other two are not.
func Resolve(flag string) (*Config, string, error) {
	if flag != "" {
		cfg, err := Load(flag)
		if errors.Is(err, ErrNoConfig) {
			return nil, flag, fmt.Errorf("--config %s: no such file", flag)
		}
		return cfg, flag, err
	}

	for _, path := range []string{LocalName, UserPath()} {
		if path == "" {
			continue
		}
		cfg, err := Load(path)
		if errors.Is(err, ErrNoConfig) {
			continue
		}
		return cfg, path, err
	}
	return Default(), "", nil
}

// UserPath is $XDG_CONFIG_HOME/skill-review/config.toml, or the empty string if neither
// that nor a home directory is set.
func UserPath() string {
	dir, err := os.UserConfigDir()
	if err != nil {
		return ""
	}
	return filepath.Join(dir, userDir, userName)
}

// Every error names the file and the key at fault: a config file is only as good as what
// it says when it is wrong.
func Load(path string) (*Config, error) {
	var cfg Config
	md, err := toml.DecodeFile(path, &cfg)
	if err != nil {
		if os.IsNotExist(err) {
			return Default(), fmt.Errorf("%w at %s", ErrNoConfig, path)
		}
		if parse, ok := errors.AsType[toml.ParseError](err); ok {
			return nil, fmt.Errorf("%s:\n%s", path, parse.ErrorWithPosition())
		}
		return nil, fmt.Errorf("%s: %w", path, err)
	}

	if keys := md.Undecoded(); len(keys) > 0 {
		return nil, fmt.Errorf("%s: unknown key %q", path, keys[0].String())
	}

	// A file that only points at an updater is a legitimate config; it just does not
	// want to change the fields.
	if len(cfg.Fields) == 0 {
		cfg.Fields = Default().Fields
	}
	if err := cfg.validate(path); err != nil {
		return nil, err
	}
	return &cfg, nil
}

// The name becomes a JSON key and a data-field attribute, so it is held to identifier
// rules rather than accepting anything the user types.
var fieldName = regexp.MustCompile(`^[a-z][a-z0-9_]*$`)

// Fields are written flat onto the thread, so a field named after one of the thread's own
// keys would overwrite it.
var reserved = []string{"id", "quote", "status", "comments", "impact"}

func (c *Config) validate(path string) error {
	seen := map[string]bool{}
	for i, f := range c.Fields {
		where := fmt.Sprintf("%s: field %d", path, i+1)
		if f.Name != "" {
			where = fmt.Sprintf("%s: field %q", path, f.Name)
		}

		switch {
		case f.Name == "":
			return fmt.Errorf("%s: needs a name", where)
		case !fieldName.MatchString(f.Name):
			return fmt.Errorf("%s: name must be lower-case letters, digits and underscores, starting with a letter", where)
		case slices.Contains(reserved, f.Name):
			return fmt.Errorf("%s: %s is reserved by the thread itself; pick another name", where, f.Name)
		case seen[f.Name]:
			return fmt.Errorf("%s: declared twice", where)
		case len(f.Values) == 0:
			return fmt.Errorf("%s: needs at least one value", where)
		case f.Default == "":
			return fmt.Errorf("%s: needs a default, one of %s", where, strings.Join(f.Values, ", "))
		case !slices.Contains(f.Values, f.Default):
			return fmt.Errorf("%s: default %q is not one of %s", where, f.Default, strings.Join(f.Values, ", "))
		}
		seen[f.Name] = true

		if c.Fields[i].Label == "" {
			c.Fields[i].Label = f.Name
		}
	}

	if c.Updater.Skill == "" {
		return nil
	}
	name, err := UpdaterName(c.Updater.Skill)
	if err != nil {
		return fmt.Errorf("%s: updater.skill: %w", path, err)
	}
	c.Updater.Name = name
	return nil
}

// UpdaterName resolves a skill path to the name in its frontmatter, and is the same check
// `config init --updater` runs before it writes anything.
func UpdaterName(path string) (string, error) {
	if !filepath.IsAbs(path) {
		return "", fmt.Errorf("%s is not an absolute path", path)
	}

	file := path
	if info, err := os.Stat(path); err == nil && info.IsDir() {
		file = filepath.Join(path, "SKILL.md")
	}

	src, err := os.ReadFile(file)
	if err != nil {
		return "", fmt.Errorf("cannot read %s: %w", file, err)
	}
	name := skill.Name(src)
	if name == "" {
		return "", fmt.Errorf("%s has no name: in its frontmatter", file)
	}
	return name, nil
}
