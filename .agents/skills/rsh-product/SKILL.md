---
name: rsh-product
description: Shape Restish product direction, feature strategy, and CLI/developer-tool UX. Use when planning, prioritizing, designing, or reviewing a Restish feature before or during implementation; when evaluating whether a request fits Restish; when turning user feedback into a feature proposal; when checking CLI UX, help, errors, output, docs, tests, compatibility, or release readiness.
---

# Restish Product

Use this skill to keep Restish feature work anchored in user value, CLI ergonomics, and maintainable open-source strategy. Product work is not separate from implementation: it is the thread connecting the user problem, command shape, output contract, tests, docs, and release story.

For source-backed rationale and examples from product/UX references and popular open-source CLI projects, read `references/research-notes.md` when you need deeper context or are updating this skill.

## Core Stance

- Start with the user job, not the implementation hook.
- Prefer a small valuable change over a broad configurable surface.
- Treat docs, help text, errors, output shape, exit codes, and non-TTY behavior as product surface.
- Design for both interactive humans and scripted automation.
- Preserve Restish's role: an API-aware HTTP CLI that stays composable with the shell.
- Avoid building a feature just because it is easy, internally elegant, or possible from an OpenAPI field.

## Use This Skill For

- Feature discovery, product shaping, and prioritization.
- CLI command naming, flags, prompts, defaults, help, errors, and output design.
- Comparing alternate product directions for Restish.
- Reviewing a proposed change for user value, scope creep, discoverability, and compatibility.
- Writing design-doc product sections, issue proposals, release notes, or UX test plans.

## Do Not Use This Skill For

- General code review after a patch exists. Use `rsh-review`.
- Documentation-only execution once the product direction is settled. Use `rsh-docs`.
- Approved simplification or dead-code cleanup. Use `rsh-simplify`.
- Pure implementation where the behavior, UX, and scope are already explicit.

## Workflow

### 1. Frame The Opportunity

Write the smallest useful product frame before designing commands:

- **User**: Who is this for? First-time user, daily CLI user, API integrator, plugin operator, plugin author, or maintainer?
- **Job**: What is the user trying to get done outside Restish?
- **Current workaround**: What command sequence, script, config edit, docs lookup, or mental model do they use today?
- **Pain**: Is the problem failure, slowness, repeated typing, poor discoverability, unsafe behavior, bad output, weak automation, or missing capability?
- **Outcome**: What should be easier, safer, faster, or more understandable after the change?
- **Evidence**: Point to issue text, user report, failing example, design-doc gap, docs search, repeated support question, or code/test signal.

If evidence is thin, propose the lightest validation instead of inventing a large feature.

### 2. Classify The Investment

Classify the work so scope and validation match risk:

- **Reliability**: Makes existing behavior more correct, predictable, secure, or recoverable.
- **Usability**: Makes existing behavior easier to discover, understand, compose, or debug.
- **Feature**: Adds a new user-visible capability.
- **Compatibility**: Preserves or improves v1, plugin, OpenAPI, shell, or documented behavior.
- **Platform**: Improves internal architecture only when it unlocks user-facing reliability, velocity, or simplicity.

Favor reliability and usability improvements when a new feature would deepen existing confusion.

### 3. Shape The Product Bet

For meaningful changes, produce a short product note or design-doc section:

```markdown
## Product Frame

Problem:
Users need ...

Goals:
- ...

Non-goals:
- ...

Primary workflow:
1. ...

CLI surface:
- Command/flag/config:
- Defaults:
- TTY behavior:
- Non-TTY behavior:
- Output:
- Errors:

Compatibility:
- Existing behavior preserved:
- Intentional break, if any:

Validation:
- Tests:
- Docs/help:
- Manual checks:

Open questions:
- ...
```

Keep the bet rough, solved, and bounded: enough detail to reduce risk, not so much that implementation has no room to find the right shape.

### 4. Apply Restish Product Principles

Use these as decision filters:

- **API-aware, shell-native**: Generated OpenAPI commands should feel richer than raw HTTP, but generic URL requests must remain excellent.
- **Human first, machine always**: Human output may be friendly in a TTY; machine-readable output must be stable, color-free when piped, and easy to combine with `jq`, `grep`, files, and scripts.
- **Examples before reference**: Help and docs should lead with real commands and representative output.
- **Safe by default**: Destructive, auth-sensitive, TLS-sensitive, or remote state-changing flows need clear confirmation, dry-run, or explicit force paths when appropriate.
- **No mystery waits**: Network, pagination, streaming, plugin, and subprocess flows need responsive feedback or bounded timeouts.
- **Progressive disclosure**: Common workflows should be simple; advanced auth, content negotiation, output, plugin, and OpenAPI controls should be discoverable without crowding the default path.
- **Convention over novelty**: Reuse familiar flag names and CLI conventions unless Restish has a strong reason to differ.
- **Few knobs, strong defaults**: Add configuration only when users need durable variation. Do not add parsed-but-unused or speculative config.
- **Compatibility is product**: Preserve documented behavior, v1 migration promises, plugin protocol contracts, and scriptability unless the product note calls out an intentional break.
- **Docs are part of done**: User-visible behavior needs `site/` updates; significant architecture or behavior needs `docs/design/` updates.

### 5. Design The CLI UX

Check every new or changed command, flag, output mode, or prompt:

- The command name matches the user's object and action vocabulary.
- `-h` and `--help` work at the command and subcommand level.
- Top-level and subcommand help include the most common examples first.
- Required input can be supplied non-interactively; prompts are TTY-only.
- Dangerous operations have confirmation, `--dry-run`, `--force`, or equivalent safety where appropriate.
- `stdout` is reserved for primary output; diagnostics and progress go to `stderr`.
- TTY output may use color, tables, summaries, and progress; piped output must be clean and deterministic.
- JSON, YAML, raw, table, streaming, pagination, filtering, and redirect behavior remain compatible with existing output contracts.
- Errors say what failed, why it matters, and what the user can do next.
- Exit codes distinguish success from failure and preserve script expectations.
- Ctrl-C, timeouts, retries, plugin subprocesses, and partial output fail in understandable and recoverable ways.
- Config, environment variables, flags, profiles, and API-generated defaults have clear precedence.

### 6. Validate Before And After Implementation

Use validation proportional to risk:

- **Product validation**: Compare against user jobs, known feedback, current docs, and one or two realistic command transcripts.
- **Usability review**: Run a heuristic pass for status visibility, vocabulary, control/escape, consistency, error prevention, recognition over recall, efficiency, minimal output, error recovery, and help.
- **CLI contract tests**: Cover TTY vs non-TTY when behavior differs, `stdout`/`stderr`, exit codes, help text, output formats, malformed input, cancellation, auth/config precedence, and shell composition.
- **Golden output tests**: Use for intentional formatter/help changes; do not accept drift casually.
- **Docs checks**: Update `site/` for user behavior and `docs/design/` for significant subsystem behavior; link guides and reference pages both ways.
- **Compatibility checks**: Test v1-style workflows, generated OpenAPI command shape, plugin boundaries, and scripting behavior when touched.

## Output Expectations

When advising, lead with the recommendation and the reason. Then include:

- Product frame or decision.
- Tradeoffs and rejected alternatives.
- CLI UX implications.
- Validation plan.
- Docs/design-doc impact.

When implementing, leave a visible product trail in the design doc, issue note, tests, docs, help text, or final summary. Do not bury product decisions only in code.

