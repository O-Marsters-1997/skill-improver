# 3. The review target is not the skill

Status: accepted · 2026-08-02

## Context

The tool was built on the assumption that the file you read and the file the payload edits
are the same file. `Submit(cfg, outDir, skillPath, src)` used one path for both jobs and
read the skill's name out of the reviewed bytes.

That assumption is wrong for the highest-signal review there is. A skill's *output* — a
report it generated, a page it wrote — is the best evidence about whether its instructions
work, and it lives outside the skill entirely. Reviewing it has to produce a payload that
edits the skill.

## Decision

### One positional and a `--skill` flag, not an `output` subcommand

```
skill-review [--skill <path>] <target>
skill-review handoff [--skill <path>] <target>
```

The positional is the **target** — what is read and commented on. `--skill` is what the
payload edits, defaulting to the target. The existing positional named `skill` is renamed
to `target` so it does not collide with the flag.

An `output` subcommand was the alternative. It was rejected because every flag, every
route and the whole review lifecycle are identical in both cases: only two strings in the
payload differ. A second command would have been a second copy of `serve` whose only job
was to bind the same arguments differently, and `skill-review output <skill> <file>` makes
the reader guess which positional is which.

Omitting `--skill` behaves exactly as before, which is what keeps every existing
invocation, the `/skill-comments` wrapper and the README honest.

### Mode is derived from the paths, never declared

The payload gains `mode`: `instructions` when the reviewed document resolves inside the
skill directory, `output` when it does not. It is computed, not flagged.

A `--mode` flag would let the two disagree — `--skill ~/.claude/skills/ideate --mode
instructions ./report.md` is a payload that tells the updater to edit text that is not in
the file it names. There is no reading of that combination worth honouring, so the
question is not asked.

Derivation resolves symlinks on **both** sides before `filepath.Rel`. This is
load-bearing rather than defensive: `~/.claude/skills/*` is largely a symlink farm into
`~/.agents/skills/*`, so without it every skill installed the normal way computes as
`output` and every prompt asks the updater to infer instructions it could have read.

The skill's directory is `filepath.Dir` of the **resolved `SKILL.md`**, not the resolved
directory: some installs link the skill directory and some link the `SKILL.md` into a
directory that is otherwise real, and only the former survives resolving the directory.

The distinction matters because it changes what the updater is being asked to do. In
`instructions` mode a suggestion is about the text it is anchored to. In `output` mode it
is an observation about a document the skill *produced*, and the updater has to infer
which instruction allowed it and edit the `SKILL.md` — never the reviewed file. The prompt
says so in as many words.

### `file` is absolute, and lives on the suggestion

Each suggestion carries `file`, the absolute path of the document its thread was anchored
in. Absolute because the updater runs wherever it runs; nothing guarantees it shares a
working directory with the reviewer, and a payload that has to be resolved against an
unrecorded `cwd` is a payload that silently edits the wrong file.

It sits on the suggestion rather than on the payload because a review spans several files
before long, and on `comments.Thread` it would be redundant: file identity is implicit in
whichever file the `mc` markers live in. It is stamped on at Submit time from the document
the threads were parsed out of, so it cannot drift from where the comment actually is.

`file` and `mode` join `id`, `quote`, `status`, `comments` and `impact` in
`config.reserved`, because configured fields are written flat and a `[[field]]` named
`file` would overwrite it.

### `skill_path` is the path the reviewer gave, made absolute

`Submit` resolves the skill to its `SKILL.md` internally — to read the name and to derive
the mode — but the payload carries the path as given, absolutised and nothing more. A
reviewer who passes `~/.claude/skills/ideate` should see that in the payload, not the
`~/.agents/skills/ideate/SKILL.md` behind the symlink, which they have never heard of.

An explicit `--skill` is checked before anything is served: it has to resolve to a
`SKILL.md` with a `name:`, or the command fails naming the flag. Omitting it cannot fail,
because then it is the target and the target has already been read.

## Consequences

`skill_name` now comes from `<skill>/SKILL.md` rather than from the reviewed bytes. A
target that is not a skill and has no `--skill` has no name, which is exactly what reading
its bytes used to give.

`Submit` takes `docs []handoff.Doc` rather than one `src []byte`. Only one is passed
today; the signature is in the shape directory targets need so it does not change twice.

`server.Server` holds `skill` alongside `path`. The page still shows the target, which is
the file you are looking at.
