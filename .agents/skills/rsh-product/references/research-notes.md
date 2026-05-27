# Research Notes

These notes summarize source-backed product and UX patterns useful for Restish and CLI tool work. Load this file when you need rationale, examples, or source links.

## General Product Development

- Product discovery should validate that an idea is valuable, usable, feasible, and strategically aligned before the team commits to building it. Atlassian frames discovery as continuous, collaborative, data-driven work that connects ideas, customer evidence, prioritization, and delivery status.
  Source: https://www.atlassian.com/agile/product-management/discovery

- Prioritization needs a balance of structured scoring and judgment. RICE, impact/effort, opportunity scoring, and similar frameworks are useful when they clarify tradeoffs rather than replace thinking. Atlassian also describes RUF: reliability, usability, and new features, which is a useful balance for mature developer tools.
  Sources: https://www.atlassian.com/software/jira/product-discovery/resources/handbook/prioritization and https://www.atlassian.com/agile/product-management/prioritization-framework

- Shape Up is useful for feature shaping: define an appetite, make the work rough but bounded, bet on meaningful scoped outcomes, and ask whether the problem matters, the appetite is right, the solution is attractive, and the timing/team fit.
  Sources: https://basecamp.com/shapeup/ and https://basecamp.com/shapeup/2.3-chapter-09

- Continuous discovery habits emphasize clear outcomes, regular user conversations, opportunity mapping, multiple solution ideas, assumption identification, and assumption testing.
  Source: https://learn.producttalk.org/cdh-master-class

## Product Strategy In Open Source Projects

- GitLab makes product direction explicit through direction pages, category strategies, monthly review, customer discovery summaries, and a product development flow that connects problem validation, solution validation, planning, build, verify, launch, and feedback. Especially relevant: "delivery follows discovery", minimal valuable change, iteration, discoverability without being annoying, convention over configuration, and small primitives.
  Sources: https://handbook.gitlab.com/handbook/product/product-processes/ and https://handbook.gitlab.com/handbook/product/product-principles/ and https://handbook.gitlab.com/handbook/product-development-flow/

- GitHub's public roadmap uses issues, labels, release phases, product areas, and shipped links to communicate direction while explicitly treating roadmap items as plans rather than promises. This is a good model for transparent expectations.
  Source: https://github.com/github/roadmap

- Rust project goals make work explicit through time-boxed goal periods, themes, flagship goals, points of contact, team asks, champions, and work plans. This is a strong pattern for community-scale projects: make ownership and required support visible.
  Source: https://rust-lang.github.io/rust-project-goals/index.html

- Visual Studio Code's roadmap looks 12 to 18 months out, builds on prior learning and user feedback, and reserves the right to add or drop topics as learning continues. This is a useful reminder to treat roadmaps as living artifacts.
  Source: https://github.com/microsoft/vscode/wiki/Roadmap

## Product Strategy In Popular CLI Tools

- The Command Line Interface Guidelines synthesize modern CLI best practices: use argument parsers, zero/non-zero exit codes, `stdout` for primary output, `stderr` for diagnostics, `-h`/`--help`, concise default help, examples first, TTY-aware human output, machine-readable output when useful, helpful errors, safe flags for dangerous operations, standard flag names, prompt only in TTYs, progress for slow work, timeouts, Ctrl-C behavior, and privacy-respecting analytics.
  Source: https://clig.dev/

- GitHub CLI positions `gh` around reducing context switching: bring PRs, issues, Actions, and GitHub workflows to the terminal where developers already work. Its extension model lets users add commands, and its docs call out non-interactive scripting, JSON output, `gh api`, and `--jq` for automation.
  Sources: https://docs.github.com/en/github-cli and https://docs.github.com/en/github-cli/github-cli/creating-github-cli-extensions

- GitHub CLI telemetry documentation explains the product rationale for usage data, how to inspect what would be sent, opt-out paths, and boundaries around extensions. For Restish, any telemetry-like product decision should be explicit, privacy-preserving, and easy to disable.
  Source: https://cli.github.com/telemetry

- Kubernetes documents `kubectl` around clear command families: manage resources, inspect state, debug, operate clusters, and automate. It explicitly recommends declarative `apply` for production and imperative commands for development/experimentation. Restish can use the same "workflow family" thinking for HTTP, generated API commands, auth, filtering, pagination, plugins, and debugging.
  Sources: https://kubernetes.io/docs/concepts/overview/kubectl/ and https://kubernetes.io/docs/reference/kubectl/introduction/

- Homebrew uses anonymous analytics to prioritize fixes, support decisions, and deprecations, but documents the why, what, retention period, notice, and opt-out. It is a useful open-source example of product feedback loops with transparency.
  Source: https://docs.brew.sh/Analytics

- ripgrep succeeds partly through strong defaults: recursive search, respecting ignore files, clear output with line numbers and color in terminals, filtering controls, stdin support, and a quick path from first use to advanced configuration.
  Sources: https://ripgrep.dev/docs/getting-started/ and https://ripgrep.dev/docs/guide/

## General UX For CLI And Developer Tooling

- Nielsen Norman Group's ten usability heuristics transfer well to CLI design: status visibility, user vocabulary, control and escape, consistency, error prevention, recognition over recall, expert efficiency, minimalist output, error recovery, and help/docs.
  Source: https://www.nngroup.com/articles/ten-usability-heuristics/

- Error guidance maps directly to CLI diagnostics: avoid premature or noisy errors, provide constraints before failure, reserve alarming treatment for real problems, use plain language, state the precise problem, and offer a constructive next action.
  Source: https://www.nngroup.com/articles/error-message-guidelines/

- Usability testing can be lightweight: recruit realistic users, ask them to perform realistic tasks, observe behavior, avoid leading questions, and use 5 to 8 participants for qualitative testing. For CLI tools, a command transcript, screen share, or local shell session is often enough.
  Source: https://media.nngroup.com/media/articles/attachments/Usability-Testing-101_SizeA4.pdf

- GOV.UK's user research guidance emphasizes understanding who users are, what they are trying to do, how they work today, inclusive research, continual research, team participation, and sharing findings.
  Sources: https://www.gov.uk/service-manual/user-research and https://www.gov.uk/service-manual/user-research/how-user-research-improves-service-design

## Useful Checks For Restish Feature Work

- Does this solve a user job or only expose an internal capability?
- Is the primary workflow command-first and example-backed?
- Does the default fit the common case without harming scripts?
- Are human and machine output both intentionally designed?
- Are `stdout`, `stderr`, exit codes, and colors correct for pipes?
- Are prompts, progress, and confirmation TTY-aware?
- Can the same workflow run in CI without hanging?
- Are auth, TLS, profile, config, and environment precedence obvious?
- Are generated OpenAPI commands, generic URL requests, and plugin workflows still coherent together?
- Are pagination, streaming, filtering, and formatters compatible with existing output contracts?
- Does the error message help the user recover without reading source code?
- Is the feature discoverable through help, examples, docs, completions, or naming?
- Does the implementation avoid speculative config and feature bloat?
- Are docs and design records updated where behavior becomes user-visible or architecture-significant?

