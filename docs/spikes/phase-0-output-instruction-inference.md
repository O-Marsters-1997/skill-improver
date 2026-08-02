# Phase 0 — falsify the output→instruction inference

**Issue:** #3 · **Date:** 2026-08-02 · **Verdict: GO.** The inference holds, and holds on a naive
observation once the review is submitted as a set. #8–#12 proceed unchanged, with one sentence added
to #8's scope.

Every phase after this one assumes `skill-updater` can turn *"this paragraph of the output is
wrong"* into *"therefore this instruction is wrong."* Nothing verified that. This is the check.

*Terminology: this document says **observation** for what a reviewer writes about a passage. In the
payload it is a `suggestion` and in the file it is a `thread`, per the glossary in `docs/CONTEXT.md`.
Same thing, named from the reviewer's side.*

## Setup

- **Skill under test:** `to-tickets` (`my-claude-code/skills/workflow/to-tickets/SKILL.md`, 152
  lines).
- **Output under review:** GitHub issues #3–#12 of this repo — the ten tickets `to-tickets` filed
  from `plans/target-is-not-the-skill.md` on 2026-08-02. Concatenated verbatim into one Markdown
  file, because `skill-review` serves files and this output was not one (see *Gaps found*).
- **Interface:** the Phase-1 payload shape from the plan — `mode: "output"`, per-suggestion `file`,
  and the `cause` field from #4 — so the experiment tests the interface the feature will actually
  produce, not today's.
- **Prompt:** the output-mode wording #8 specifies for `handoff.Prompt`:

  > The suggestions below are observations about a document the `to-tickets` skill PRODUCED — not
  > about the skill's own instructions. Each one names a specific passage of that output and says
  > what is wrong with it. Your job is to work out which instruction in the skill's SKILL.md caused
  > each observed problem, and edit that instruction. Edit the SKILL.md at `skill_path`. Do NOT edit
  > the reviewed document at `file`. Apply the suggestions in priority order, high first.

- `skill-updater` ran in a fresh context against a scratch copy of the skill, stopped after its
  step 3. No PR was opened; the real skills repo was not touched.

Each observation names a passage and says what is wrong with **the output**. None names a line of
`SKILL.md`, mentions the issue template, or prescribes a fix — otherwise the test measures nothing.
See *Limitations* for the two places that guard is weaker than it looks.

Three runs, in the order they were done:

| Run | Input | Question it answers |
| --- | --- | --- |
| 1 | 3 diagnosed observations, one payload | Can it trace a well-written observation at all? |
| 2 (control) | 1 naive reaction, alone | Does it work on the kind of comment a real reviewer writes? |
| 3 | all 4 together, one payload | Does submitting them as a set fix what run 2 broke? |

## The three observations (run 1)

**1 — high / instructions / cause: instructions**

> Issue #8 opens with: "## Source plan — `./plans/target-is-not-the-skill.md` — Phase 1. Read the
> plan's **Technical design decisions** first: payload contract, key models, mode derivation." All
> ten issues open with that same relative path. `plans/` is in this repo's `.gitignore` and the file
> is in no commit, so it exists only on the laptop that filed the issues. #8, #9 and #10 go further
> and make reading it a prerequisite for starting — #10 says "Read that section in full before
> starting; the offset trade-off is the whole difficulty." Anyone who picks these up off the board
> cannot open the thing the first line tells them to read.

> [!NOTE]
> **Correction, kept because the observation is the artefact.** "`plans/` is in this repo's
> `.gitignore`" is true only of the filing machine's *uncommitted* `.gitignore`; at every commit the
> file is `bin/` and `.skill-review/` alone, so `plans/` is untracked rather than ignored. The
> conclusion is unaffected — the plan is in no commit either way. Worth noting that the observation
> carried a factual error and the inference still landed on the right instruction, because the
> inference keys off the *symptom* in the output, not the reviewer's explanation of it.

**2 — medium / instructions / cause: instructions**

> Issue #7's acceptance criteria include "- [ ] The finding is written into this issue as a comment
> before it is closed" and "- [ ] No spike code is merged to `main`". Issue #3's include "- [ ] If
> the verdict is negative, #8–#12 are re-scoped or closed rather than started". None of these
> describe the finished work. Two are instructions about how to close the ticket, and the third
> cannot be ticked when the work is done because it depends on edits to five other tickets. They sit
> in the same checklist as genuinely checkable items like "- [ ] `just check` passes", so nothing
> distinguishes the boxes that describe the deliverable from the boxes that describe the author's
> intentions.

**3 — medium / structure / cause: instructions**

> Issue #6's "## User stories addressed" section reads in full: "- Not a user story — a safety
> default the plan pulls forward from Phase 3". Issue #12's reads "- Not a user story — the
> documentation debt the whole arc creates". Issue #7's reads "- Multi-format file explorer —
> de-risks the renderer before it is built", which is a rationale rather than a user story but is
> not flagged as one. So three of the ten issues carry a section whose only content contradicts or
> stretches its own heading.

## Run 1 — three diagnosed observations

All three traced to a specific instruction. **Mergeable, with one nit.**

- **Observation 1.** It reached a better answer than the obvious one. The obvious cause is the
  template line prescribing the local path (`"## Source plan" + ./plans/<file>.md`). It instead
  blamed *"Reference specific sections of the source plan rather than duplicating content"* (line
  112) for turning provenance into a dependency, and constrained the path line rather than deleting
  it — the trail stays, the dependency goes.
- **Observation 2.** The cause is an *absence*: three unlabelled placeholder boxes and no definition
  of a criterion. Diagnosing a missing instruction is harder than editing a bad one. (Discounted
  somewhat — see *Limitations*.)
- **Observation 3.** Correct and unremarkable — the section was unconditional, so it added an
  escape hatch, modelled on the one **Blocked by** already has.
- **Nit, present in all three runs:** the insertions land *inside* the `<issue-template>` block,
  which is the literal body a future run copies into an issue. The existing template already mixes
  guidance with content, so this is in keeping, but the new blocks are longer and read as
  instructions to the author. Move them outside the tags before merging. A placement nit, not a bad
  inference.

## Run 2 (control) — one naive reaction, on its own

The three above were written by someone who had read the `SKILL.md`. A real reviewer of an output
usually has not. So: fresh context, fresh copy of the skill, one deliberately undiagnosed reaction,
and an explicit escape hatch — *"if you judge that the observation cannot be traced to any
instruction, say so and change nothing — do not invent an edit to have something to show."*

> Issue #8 is exhausting. Its "What to build" is nine bullets of function signatures, struct fields
> and file:line references […] then eleven acceptance boxes, then a list of tests that will break.
> #9 and #10 are the same size. I read #8 twice and still didn't know where to start.

It traced correctly — to line 112 again, plus the criterion placeholders — but the **edit** was not
mergeable: it invented "at most five bullets" and "three to five criteria", two numeric caps derived
from one complaint about one ticket. It also declined to touch `<vertical-slice-rules>` on the
stated grounds that doing so would be guessing, which is the right instinct applied in one place and
absent in the other.

**And it contradicted run 1 on the same line.** Run 1 rewrote line 112 to *inline* the design
decisions; run 2 rewrote it to *point at the plan and not restate signatures*. Two honest reviews of
the same output, opposing instructions for the same line.

So the control separates two things the ticket's question runs together. The **inference** held —
it found the right instruction from a pure reaction. The **edit calibration** did not.

## Run 3 — all four together, as one review

`skill-review` submits every open thread in one payload, so run 2's isolation is not how the feature
will be used. Run 3 tests that: the same four observations, one payload, the naive one carried at
`cause: unknown` so the updater had to decide for itself. The prompt gained one sentence — *"weigh
them against each other, not one at a time"* — plus the escape hatch from run 2.

Result, +18/-1:

```diff
@@ -74,6 +74,7 @@ <vertical-slice-rules>
 - Prefer many thin slices over few thick ones
+- A slice needing more than ~8 acceptance criteria, or a **What to build** that can't be read once and acted on, is too thick — split it rather than writing a longer ticket
 - Respect the plan's **deep modules** — ...
 </vertical-slice-rules>

@@ -107,12 +108,23 @@ <  Conversation: "## Source conversation" + one-line note ...>

+The source reference is **provenance, not a prerequisite**. `./plans/` is commonly gitignored and a
+pasted conversation exists nowhere at all, so the source usually lives only on the machine that ran
+this skill. Never write "read the plan first" or make opening the source a precondition for
+starting.
+
 ## What to build

-A concise description of this vertical slice. Describe the end-to-end technical behaviour — what gets created, called, stored, and returned. Reference specific sections of the source plan rather than duplicating content.
+A concise description of this vertical slice. Describe the end-to-end technical behaviour — what gets created, called, stored, and returned. Restate inline the decisions this slice depends on — data shapes, contracts, the files it touches — so the ticket is startable by whoever grabs it off the board with no access to the source. Restate what the work needs, not the whole design; if that is too much to read once, the slice is too thick — go back and split it.

 ## Acceptance criteria

+Each box is an observable property of the finished work, checkable against this ticket alone. Not
+process steps ("post the finding as a comment before closing", "no spike code merged to `main`"),
+and not conditions on other tickets ("if the verdict is negative, #8–#12 are re-scoped"). Those are
+instructions to the author, and a checklist mixing them with deliverables can't tell a reviewer
+whether the ticket is done — put them in **What to build**.
+
 - [ ] Criterion 1

@@ -130,6 +142,10 @@ - User story 7

+Omit this section entirely when the slice serves no user story — a spike, a safety default, a docs
+cleanup. Never write a placeholder that denies its own heading ("Not a user story — ...") and never
+stretch a rationale to fill it.
+
 </issue-template>
```

**Mergeable, same placement nit.** Three things make this the strongest run:

1. **It found the contradiction itself and resolved it correctly.** Unprompted beyond "weigh them
   against each other", it reported that observation 1 and the naive one pull line 112 in opposite
   directions — one says the ticket carries too little, the other too much — and resolved it by
   making the criterion *sufficiency to start* rather than volume, then routing the overflow case to
   slice size instead of trimming content. That is a better answer than either isolated run reached,
   and it is the answer I would have wanted.
2. **The naive observation landed somewhere useful.** Run 2 refused to touch
   `<vertical-slice-rules>` because doing so from one complaint was guessing. With three other
   observations for context, run 3 went to line 75 — *"Prefer many thin slices over few thick
   ones"* — and gave the preference an operational trigger, instead of inventing bullet-count caps
   in the template. One number is still invented (`~8` criteria), but it is hedged, it is a split
   trigger rather than a hard cap, and #8's eleven boxes would have tripped it.
3. **`cause: unknown` was used as designed.** It chose `instructions` over `execution` with a stated
   argument — #8 is not a verbose write-up of a thin slice, it is genuinely thick — rather than
   defaulting.

## Findings

1. **The inference holds.** Eight for eight across three runs, every observation traced to a
   specific instruction, including one that was a missing instruction rather than a wrong one, and
   one written as a pure reaction with no analysis. This was the thing that might have failed.
2. **Inference and edit calibration are separate risks.** Run 2 shows the inference surviving a
   naive observation while the edit over-reaches into invented specifics. Only the second is
   fragile, and only when an observation is submitted alone.
3. **Reviewing as a set is what fixes it — tested, not assumed.** Run 3 handled the same
   contradiction that run 2 created, and produced the best edit of the three. The feature already
   submits the whole thread set in one payload, so this is a property of the design. The failure
   mode is piecemeal submission across sessions, which the archive boundary already discourages.
4. **The updater will invent a cause if not told otherwise.** Runs 2 and 3 both carried a hand-added
   "report it rather than invent an edit" sentence, and run 2 still over-reached. Without it, worse.
   That sentence belongs in the shipped prompt.

## Limitations

Recorded because the verdict is a go/no-go and the case against it should be legible.

- **`cause: instructions` pre-asserts half the proposition.** All three run-1 observations carried
  it, which asserts *that* an instruction is at fault before the updater looks. It does not leak
  *which* — the part actually under test — but it narrows the question. Run 3's fourth observation
  carried `cause: unknown` and still resolved correctly, which is the mitigating evidence. A real
  reviewer would set this field, so the narrowing is realistic rather than artificial.
- **Observation 2 hands over the distinction its fix uses.** "Nothing distinguishes the boxes that
  describe the deliverable from the boxes that describe the author's intentions" describes the
  output's defect, but it also supplies the concept the edit turns into a rule. Treat it as weaker
  evidence than observations 1 and 4.
- **n = 1 skill, one model, three runs.** No claim about variance, and no claim that this transfers
  to a skill whose output is code rather than prose.
- **The reviewer wrote the observations and judged the edits.** Unavoidable at this scale; it means
  "would I merge this" is one person's bar.

## Decisions

- **#8–#12 proceed as written.** Nothing is re-scoped or closed.
- **#8 gains one sentence.** Its `handoff.Prompt` bullet already says the updater must infer which
  instruction caused each observation. Add: in `output` mode the prompt must also tell the updater
  to weigh the suggestions as one review rather than one at a time, and to report an observation it
  cannot trace rather than invent an edit for it. Prompt text only, no interface change. **Recorded
  here, not applied** — editing #8's body is #8's business, so someone must carry this across.
- **#4's `cause` field is confirmed.** Run 3 is the evidence: `unknown` was resolved with a stated
  argument rather than a default, which is the behaviour the field exists to enable.

## Gaps found, not blocking

- **`to-tickets`' output was GitHub issues, not files.** `skill-review` serves files, so this spike
  concatenated ten issue bodies into a Markdown file by hand. No ticket covers non-file outputs.
  Fine for now — the plan's motivating example (`ideas/reports/2026-08-02-ideate.md`) is a file —
  but it bounds what "review a skill's output" currently means.
- **`skill-updater` step 1 globs `<source-repo>/skills/*/SKILL.md`.** The real repo nests one level
  deeper (`skills/workflow/to-tickets/`), so discovery finds nothing and only the programmatic path,
  which skips discovery, works today. Not this repo's bug; worth telling the skill's author.

## Reproducing

Nothing was committed to the skills repo and no PR was opened. Copy
`skills/workflow/to-tickets/` into a scratch git repo, build a payload from the observations above
with `skill_path` pointing at the copy, and run `skill-updater` with the output-mode prompt, stopping
after its step 3. Run 3 is the one to reproduce: all four observations in one payload, the fourth at
`cause: unknown`, with the "weigh them against each other" and "report rather than invent" sentences
appended to the prompt.
