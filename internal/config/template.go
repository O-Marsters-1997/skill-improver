package config

import (
	"fmt"
	"strings"
)

// Template is the file `config init` writes: the built-in defaults spelled out, so the
// starting point is editable rather than invisible. A round-trip test keeps it honest —
// what this writes must load back as Default().
func Template(updaterSkill string) string {
	var b strings.Builder

	b.WriteString(`# skill-review configuration.
#
# Every value below is what skill-review uses when there is no config file at all,
# so deleting this file changes nothing.

# Each [[field]] is one control on a thread card and one key on every suggestion in
# the handoff payload. Delete a block to drop the field; add one to introduce your
# own. Suggestions are sorted by the FIRST field, in the order its values are listed.
`)

	for _, f := range Default().Fields {
		fmt.Fprintf(&b, `
[[field]]
name    = %q
label   = %q
values  = [%s]
default = %q
`, f.Name, f.Label, quoteList(f.Values), f.Default)
	}

	b.WriteString(`
# The skill that applies the suggestions. An absolute path to a skill directory or a
# SKILL.md; its name is read from the frontmatter and used in the handoff prompt and
# on the Submit button.
#
# Leave this out to get the built-in prompt, which spells the instructions out
# instead of naming a skill.
`)
	if updaterSkill == "" {
		b.WriteString("# [updater]\n# skill = \"/absolute/path/to/skill-updater\"\n")
		return b.String()
	}
	fmt.Fprintf(&b, "[updater]\nskill = %q\n", updaterSkill)
	return b.String()
}

func quoteList(values []string) string {
	quoted := make([]string, len(values))
	for i, v := range values {
		quoted[i] = fmt.Sprintf("%q", v)
	}
	return strings.Join(quoted, ", ")
}
