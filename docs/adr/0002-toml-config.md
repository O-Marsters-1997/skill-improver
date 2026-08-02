# 2. Configure the comment schema and the updater in TOML

Status: accepted · 2026-08-02

## Context

The tool shipped with one person's review lifecycle compiled in. A comment could be given a
`priority` from three values and a `category` from six, and the payload always went to a
skill called `skill-updater`. None of that is wrong, but none of it is inherent either: a
different reviewer wants a severity and an area, or one field, or a different skill, or no
skill at all.

Worse, the taxonomy existed in four places — a `var` block in `internal/handoff`, two
literal arrays in `app.js`, the struct tags on `comments.Thread`, and a prose restatement in
the `/skill-comments` slash command. They agreed only by hand.

The requirement is therefore one description of the review shape, read at startup, that
drives the sidebar controls, the persisted suggestions, and the prompt.

## Decision

### TOML, via `github.com/BurntSushi/toml`

The file is edited by a human and read by a program, which rules out JSON (no comments — and
a config whose defaults cannot be explained inline is a config nobody edits). YAML would need
a much larger dependency to express something with no nesting to speak of.

The library earns its place on error reporting, which is the whole point of a config file
that can be wrong. `ParseError.ErrorWithPosition()` prints the offending line with a caret
under the column; `MetaData.Undecoded()` turns a typo'd key into an error rather than a
silently ignored line. Hand-rolling a parser for a format with strings and string arrays is
about a hundred lines, but hand-rolling *those two diagnostics* is not, and without them a
misconfigured file is worse than none. One dependency, no transitive ones — the same bargain
as [ADR-0001](0001-goldmark.md).

A missing config file is not an error. `config.Default()` is the shape the tool shipped with,
so an unconfigured machine is byte-for-byte unchanged. A config file that *exists* and will
not load is fatal: falling back silently would hand a reviewer the wrong controls without
saying so.

### Fields are stored flat on the thread, not nested

A thread is one `<!--mc:t {JSON}-->` line. The configured fields are written as keys directly
on that object —

```
{"id":"a","quote":"…","status":"open","comments":[…],"category":"tools","priority":"high"}
```

— rather than under a `"fields"` key of their own. Nesting would have been less code: no
custom `MarshalJSON`, no reserved-name rule. It was rejected because every `SKILL.md` already
written would have stopped parsing, and the whole architecture rests on the file being the
only state. Flat also keeps faith with the borrowed `mc` format, where unknown keys are meant
to be ignored rather than owned.

Two consequences follow, both deliberate. Field names are checked against `id`, `quote`,
`status`, `comments` and `impact`, because a field called `status` would overwrite the
thread's own. And unmarshalling keeps *every* unrecognised string key, not just the ones the
config declares — so a value for a field since removed survives in the file instead of being
erased the next time an unrelated thread is written.

### The first field is the sort key

Suggestions are ordered by the first `[[field]]`, in the order its values are listed. An
explicit `sort = true` key was the alternative; a convention costs no config, and the field
you would rank by is the one you would list first anyway.

### `handoff.Submit` gives up purity on purpose

`internal/handoff` was pure. Moving the pending-file write into it breaks that, and the
alternative — leaving it in `internal/server` — was rejected because the `handoff` subcommand
needs the identical behaviour, and the previous attempt at "two paths that must agree" is
exactly the drift this ADR is cleaning up. `Build` stays pure and table-tested; `Submit` is
the single documented exception.

## Consequences

The four copies of the taxonomy become one. `app.js` builds its controls from what
`/api/doc` serves, and `/skill-comments` shells out to `skill-review handoff` instead of
restating the rules in English.

`skill-review` grows subcommands and moves to `urfave/cli/v3` to hold them. `serve` is the
default command, so `skill-review path/to/SKILL.md` still works.

Three dependencies link into the binary rather than one. `go.mod` lists no others;
`go list -m all` shows testify and friends only because they are test dependencies of
`urfave/cli`, and `go version -m` on the built binary confirms they do not ship.

Changing the schema does not migrate existing files, and does not need to.
