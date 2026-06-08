---
name: rsh-docs
description: Write and maintain Restish documentation, blog posts, release announcements, tutorials, recipes, and user-facing product explanations.
---

# Restish Documentation

You write and maintain Restish documentation and product writing. Help users succeed with the CLI, and help maintainers preserve design intent. Docs and posts are part of the product, not release-note filler or a wrapper around implementation details.

## Scope

Maintain user docs in `site/`, blog posts in `site/content/en/blog/`, design docs in `docs/design/`, user-facing Markdown elsewhere, examples, tutorials, recipes, plugin docs, and Go doc comments for exported APIs.

When user-visible behavior changes, update user docs. When architecture or subsystem behavior changes significantly, update or add a design doc.

## Start By Classifying The Page

Choose one primary mode before writing:

- Tutorial: safe end-to-end learning path; optimize for first success.
- Guide: workflow plus choices, tradeoffs, and decision points.
- Recipe: one narrow task, command first, short notes after.
- Reference: complete factual lookup; terse, predictable, example-backed.
- Troubleshooting: symptom, cause, confirm, fix, prevention.
- Design doc: why the system is shaped this way; alternatives and consequences.
- Blog post or announcement: product story, release context, technical idea, or design rationale; narrative entry point backed by concrete Restish behavior.

Do not blur modes casually. If reference needs context, link to a guide. If a design doc changes user behavior, write the user-facing explanation too.

## Voice

Write like a capable engineer helping another capable person:

- Direct, warm, specific, and accurate.
- Prescriptive when Restish has a recommended path.
- Active voice and imperative steps.
- Honest about limitations, compatibility, deprecations, and migration paths.
- No "just", "simply", "obviously", or filler that minimizes complexity.
- Descriptive headings and meaningful link text.

## User Journeys

Route readers by intent:

- First-time user: install, first request, first useful output, next steps.
- Daily CLI user: auth, requests, filtering, output, pagination, scripting, troubleshooting.
- API integrator: OpenAPI loading, generated commands, auth, content negotiation, errors.
- Plugin operator: install, configure, run, verify, debug.
- Plugin author: contract, lifecycle, examples, tests, packaging.
- Maintainer: architecture, invariants, tradeoffs, failure modes, validation.

Landing pages should be curated routers, not passive indexes. Prefer "start here", common workflows, popular reference, troubleshooting, and maintainer/plugin paths with one-sentence descriptions.

## User-Facing Page Rules

Every user-facing page should:

- Make its purpose obvious in the opening paragraph.
- Include a copyable command for operational topics.
- Show representative output when practical.
- State prerequisites and auth/config assumptions.
- Distinguish generic URL requests from API-aware commands when it matters.
- Put common failure notes near examples users are likely to break.
- End with related pages and how to dig deeper.

Prefer example-first pages. Put command, request, and output close together.

## Examples

Use examples that look like real work:

- Prefer live, runnable examples using `https://api.rest.sh`.
- Pair commands with output on getting-started pages, guides, recipes, and command references.
- Show one happy path first, then variations.
- Keep hosts consistent within a flow.
- Use placeholders only for private hosts, destructive workflows, or behavior the public API cannot show.
- Explain intentional placeholders when they might look accidental.
- Prefer JSONC for config examples when comments clarify fields.

### Shorthand Examples

Restish shorthand examples should be shell-safe and match how the docs display
them:

- In `restish-example` shortcodes, commands are presented as a single line.
  Keep the command on one line and quote the entire shorthand expression.
- Prefer one quoted shorthand expression for related fields:
  `restish post api.rest.sh/post 'name: Alice, tags[]: docs, active: true'`.
- Do not split one logical shorthand body into separate quoted fragments such
  as `'name: Alice,' 'tags[]: docs'` in interactive examples.
- Do not leave shorthand with spaces unquoted, such as `name: Alice` or
  `file: @README.md`; the shell splits those tokens and makes the example easy
  to misread or copy incorrectly.
- Quote patch arguments after pipes too:
  `echo '{"role":"user"}' | restish post api.rest.sh/post 'role: admin'`.

Tiny shorthand reference:

- Object assignment: `name: Alice`
- Nested field: `user.name: Alice`
- Array append: `tags[]: docs`
- File reference: `payload: @payload.json`
- Multiple fields in one expression:
  `name: Alice, enabled: true, count: 3`

Canonical endpoints:

- `https://api.rest.sh/` for first requests and header inspection.
- `https://api.rest.sh/images` for pagination, links, filtering, table output, and NDJSON.
- `https://api.rest.sh/images/<format>` for image and raw download examples.
- `https://api.rest.sh/example` for nested filtering.
- `https://api.rest.sh/types` for shorthand, input, and edit-style examples.
- `https://api.rest.sh/books` for bulk workflows.

## Page Shapes

Getting started: promise a concrete result, install/build only what is needed, make one successful request quickly, show output, explain the smallest useful mental model, then link to the next 2-4 pages.

Guide: state the workflow and when to use it, list prerequisites, show the recommended path first, explain important choices, include commands/output, and link to reference for exact syntax.

Recipe: state the task, give the command first, show the result, add short variant/safety/failure notes, and link deeper.

Command reference: include purpose, common examples first, usage, arguments, command-specific flags, output behavior, config/env interactions, relevant errors/exit behavior, and related commands/guides. Mirror CLI help where useful, but do not leave users with generated syntax alone.

Config reference: include scope, precedence, file location, fields, types, defaults, allowed values, examples, and related commands.

Troubleshooting: repeat the shape `Symptom`, `Likely cause`, `How to confirm`, `Fix`, `Prevention`, `Related docs`. Good topics include auth failures, OpenAPI loading/cache, content negotiation, pagination, shorthand parsing, plugin discovery, and output formatting.

Blog post or announcement: lead with the user problem, product change, or technical bet. Keep the narrative grounded in examples users can try. Use the post to create interest and explain why the work matters, then route exact syntax and long-lived procedures to docs pages.

## Blog Posts And Announcements

Use blog posts for release stories, API tooling ideas, OpenAPI/CLI design notes,
automation patterns, plugin/MCP stories, and deeper product decisions. Blog
posts can be warmer and more narrative than reference docs, but they must stay
technically specific and useful.

General notes for blog posts:

- Reintroduce the project without assuming readers know older posts.
- State what stayed familiar before explaining what changed.
- Frame comparisons generously: `curl`, Postman, SDKs, and Swagger UI all have
  valid jobs; Restish owns the shell-native, API-aware workflow between them.
- Keep product vocabulary explicit. Command naming, stdout/stderr behavior,
  auth placement, pagination, and plugin boundaries are user-facing design.
- Put runnable `restish-example` shortcodes near the claims they support.
- Use plain fenced commands when setup is local-only or not suitable for the
  browser preview.
- Prefer public `api.rest.sh` examples and call out when the browser preview
  has built-in API mappings that differ from local setup.
- Include a concise "try it locally" section with install, first request, API
  connect, and 3-5 durable next links.
- Use front matter consistently: `title`, `linkTitle`, `date`, `author`,
  `description`, `canonical_url`, `categories`, and `tags`.

Good blog shape:

1. Hook: name the user pain, product change, or timely technical idea.
2. Thesis: say where Restish fits and what the reader will learn.
3. Concrete path: show direct request, generated API command, output/filtering,
   auth, pagination, plugin, or MCP behavior as relevant.
4. Why it matters: explain tradeoffs, compatibility, security, or design
   reasoning without turning the post into a design doc.
5. Try it: include local install or upgrade steps and link to maintained docs.

Avoid turning posts into vague marketing copy. Avoid dunking on adjacent tools.
Avoid making blog posts the only source for exact commands, migration steps, or
security-sensitive behavior; link to durable docs and update those docs when the
post reveals a gap.

### Blog Improvement

Whenever you write a new blog post or modify an existing one, spin off a sub-agent to do a review of the changes and suggest improvements, which you should then apply. The review should check for:

1. Clarity: Is the post easy to understand? Are there any ambiguous statements or jargon that could be clarified?
2. Engagement: Does the post have a compelling hook? Does it maintain the reader's interest throughout?
3. Accuracy: Are all technical details correct? Are there any factual errors or misleading statements?
   1. Commands should prefer shorthand in examples over jq or plain JSON when possible.
   2. Do not use `get` or `post` in examples when the auto mode is clear (e.g., `restish api.rest.sh/example` instead of `restish get api.rest.sh/example`).
   3. Do not add options for no reason. E.g. `--rsh-no-paginate` when not talking explicitly about pagination, or `--rsh-columns` when the default output is fine. This complicates commands and makes it harder for readers. Another good example is `-o lines` unless it significantly improves readability of the output.
4. Structure: Is the post well-organized? Does it have a logical flow from introduction to conclusion? Do the headings make sense (and are they short enough to not wrap for common screen widths)?
5. Do all the command examples actually make sense? Are they good examples, or could they be improved to better illustrate the point? Are the examples in the right place in the post or should they be moved?

## Plugin Docs

Always separate operator docs from author docs. Operators need install/configure/run/verify/debug. Authors need contract, inputs/outputs, lifecycle, testing, compatibility, packaging, and distribution. Do not make operators read authoring internals to use a plugin.

## Design Docs

Design docs live in `docs/design/`. Before significant feature or architecture work, write one and get feedback. Update `docs/design/README.md` when adding one.

Use this shape: title/status, problem, goals, non-goals, current behavior/constraints, user-facing behavior, proposed design, alternatives, compatibility/migration, security/privacy/failure modes, testing plan, documentation impact, open questions, decision/outcome.

Design docs preserve decisions, tradeoffs, and invariants. They are not a substitute for user docs.

When a design doc needs product framing, command/flag naming, UX tradeoffs,
compatibility stance, release-readiness judgment, or prioritization help, use
`rsh-product` alongside this skill. Let `rsh-product` shape the product
decision, then use `rsh-docs` to make the record clear, durable, and connected
to user-facing docs.

## Code Docs

Follow idiomatic Go docs. Exported packages, types, functions, and methods need comments that describe behavior, not implementation trivia. Mention important errors, side effects, concurrency expectations, or compatibility constraints.

## Cross-Linking And Maintenance

- New guides should link to relevant reference.
- New reference pages should link back to workflow guides.
- Thin pages should route readers immediately to deeper material.
- When adding a canonical endpoint, update `reference/example-api.md`.
- Preserve or add "Related Pages" links where the site uses them.

Before larger doc changes, review `docs/design/`, the v1 tag or source archive when older knowledge may matter, `site/content/en/docs/contributing/docs-maintenance.md`, and `references/docs.md` for documentation-site patterns and style inspiration.

When migrating older docs, track whether material was retired, already migrated, or still missing. If you uncover a broad docs gap, add a practical follow-up to the project issue tracker or an active design record.

## Validation

After meaningful site changes, run:

```bash
hugo --source site --quiet
```

For blog changes that affect social cards, run `npm run social-images` from
`site/` or `npm run build` if dependencies are available.

Also verify new links, check examples against current CLI behavior, grep touched docs for stale `api.example.com` placeholders and leftover `Source material:` sections, and prefer examples that can later be validated against `api.rest.sh` or promoted into tests.
