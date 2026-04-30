# Documentation Research For Restish

This research compares strong documentation sites for API services and CLI tools, then turns the patterns into guidance for improving the `rsh-docs` skill. The emphasis is practical: help Restish write user-facing docs that get readers to a working request quickly, and developer-facing design docs that preserve decisions without becoming archaeology.

## Executive Summary

Great developer docs usually combine five traits:

- A fast first-success path: install, authenticate, make one request, inspect output.
- A layered information architecture: getting started, guides, recipes/how-tos, reference, troubleshooting, release/change notes.
- Page layouts that keep action close to explanation: command/API syntax, examples, parameters, responses, caveats, and next links on the same page.
- A consistent voice: direct, calm, specific, and prescriptive when a recommended path exists.
- Examples that look like real work: copyable commands, realistic inputs, representative output, and clear notes for placeholders or destructive operations.

The best pattern for Restish is not one site wholesale. Restish should combine Stripe's example-first API reference, GitHub's journey-oriented landing pages, Terraform's workflow-based CLI guidance, kubectl's dense command examples, and Diataxis' separation of tutorials, how-to guides, reference, and explanation.

## Documentation Sites Reviewed

### 1. Stripe API Docs

Source: [Stripe API Reference](https://docs.stripe.com/api?lang=curl)

Big-picture structure: Stripe separates conceptual/product docs from a deep API reference. The API reference starts with global conventions, authentication, errors, pagination, idempotency, versioning, and then moves into resource groups.

Page layout: The reference uses a resource-oriented navigation spine, endpoint sections, request/response details, code examples, and sample objects. Language selectors let the same page serve curl and SDK users. It also includes account-aware examples and test-mode framing.

Tone and language: Confident, concise, product-aware. Stripe explains the underlying HTTP model in plain terms without over-teaching REST.

Examples and interactivity: Strongest pattern is side-by-side runnable examples with request and response nearby. The examples default to curl but switch across official libraries.

User journey: Stripe gets developers from "what is the base URL?" to "make a safe test request" quickly, then scales into complete reference lookup.

Restish takeaway: Put command, request, and output together. For command/reference docs, lead with the most common form before exhaustive flags. Keep generic HTTP mechanics visible enough that users understand what Restish is doing.

### 2. Twilio Docs

Source: [Twilio Docs](https://www.twilio.com/docs), [Twilio Verify Quickstarts](https://www.twilio.com/docs/verify/quickstarts/)

Big-picture structure: Twilio organizes by product families, then provides quickstarts, SDK docs, API references, helper libraries, and support/community routes.

Page layout: Landing pages are discovery hubs. Quickstarts are language-specific and task-focused, often ending with support links and next steps. Product pages separate "get started" from deeper API details.

Tone and language: Friendly, high-touch, and encouraging. Twilio often names the business use case before showing implementation details.

Examples and interactivity: Multi-language examples are central. The quickstarts often walk through account setup, local environment setup, first API call, and validation.

User journey: Twilio is optimized for builders trying to integrate a communication workflow, not just look up endpoint fields.

Restish takeaway: For workflows like auth, pagination, uploads, and plugins, write task-first guides that name the real-world job. End pages with help paths and related docs.

### 3. GitHub REST API Docs

Source: [GitHub REST API documentation](https://docs.github.com/en/rest?tool=cli)

Big-picture structure: GitHub uses a strong landing-page pattern: start here, popular, guides, and all docs. It then divides API material into learning, auth, guides, and endpoint categories.

Page layout: Overview pages are curated. Reference pages are deeply indexed, version-aware, and connected to authentication, rate-limit, pagination, and troubleshooting docs.

Tone and language: Clear, task-oriented, and restrained. GitHub uses approachable summaries but avoids marketing voice inside reference.

Examples and interactivity: Examples frequently include curl, GitHub CLI, and SDK variants. API docs link repeatedly to best practices and security guidance.

User journey: GitHub anticipates multiple reader intents: first-time API user, script author, integration developer, endpoint lookup, and troubleshooting.

Restish takeaway: Restish docs need "Start here" and "Popular" pathways. Landing pages should not be mere tables of contents; they should route users by intent.

### 4. OpenAI Platform Docs

Source: [OpenAI Responses API Reference](https://platform.openai.com/docs/api-reference/responses/retrieve)

Big-picture structure: OpenAI connects API reference entries to related guides for concepts such as text input, structured output, tools, and conversation state.

Page layout: Reference pages include endpoint purpose, method and path, request body fields, response object fields, examples, and linked guides. The reference is dense but cross-linked to conceptual explanations.

Tone and language: Direct and product-specific. It explains new concepts in the place where developers encounter the API fields.

Examples and interactivity: API pages combine object schemas with examples. The docs make heavy use of related guide links so readers can move from field lookup to workflow understanding.

User journey: The journey is reference-forward for experienced developers, with concept links for new platform concepts.

Restish takeaway: When Restish reference pages mention behavior like shorthand parsing, request bodies, content negotiation, or MCP/plugin behavior, link directly to a guide that explains the mental model.

### 5. Slack Developer Docs

Source: [Slack Web API](https://docs.slack.dev/apis/web-api), [Slack Web API methods](https://api.slack.com/methods)

Big-picture structure: Slack organizes by platform capability: Web API, Events API, app setup, SDKs, and method reference.

Page layout: The Web API overview defines conventions first, including method URL shape, request formats, transport/security requirements, and argument handling. Method pages are lookup-oriented.

Tone and language: Conversational but still precise. Slack is willing to acknowledge platform quirks, such as an RPC-style API that is not pure REST.

Examples and interactivity: SDK pages provide installation, prerequisites, and code snippets. Method indexes are broad and searchable.

User journey: Slack orients developers around app creation and permissions before method calls.

Restish takeaway: Be honest about conceptual mismatches. If a feature is "REST-ish" rather than pure REST, explain the convention plainly and show how Restish handles it.

### 6. Notion Developer Docs

Source: [Notion API Reference](https://developers.notion.com/reference), [Notion Developer Quickstart](https://developers.notion.com/guides/get-started/quick-start)

Big-picture structure: Notion splits overview, quickstart, guides, and reference. It starts with integration concepts, permission model, and internal versus public integrations.

Page layout: Quickstarts include why a recommended path was chosen, what to know before starting, setup steps, and sample code. Reference pages begin with conventions such as base URL, auth, IDs, JSON naming, and pagination.

Tone and language: Friendly and explanatory. It makes product-specific models, such as bot users and page permissions, explicit before code.

Examples and interactivity: Examples use GitHub sample repos and visible setup steps. Notes call out common failure causes.

User journey: Notion helps readers avoid permission mistakes before they happen, then sends them into runnable examples.

Restish takeaway: Add "common failure" callouts near examples, especially for auth, API description loading, aliases, and content negotiation.

### 7. Shopify API Docs

Source: [Shopify REST Admin API reference](https://shopify.dev/docs/api/admin-rest)

Big-picture structure: Shopify is versioned and migration-aware. It clearly marks REST Admin API as legacy and routes new app developers toward GraphQL when appropriate.

Page layout: Reference pages include version selectors, filters, overview material, auth, endpoints, rate limits, status/error codes, and grouped endpoint categories.

Tone and language: Practical and directive. The docs are comfortable telling readers when an API is legacy and what path to take instead.

Examples and interactivity: Strong filtering and versioning controls help readers find the right endpoint for their app generation.

User journey: Shopify balances lookup with lifecycle warnings: latest version, legacy status, migrations, and deprecation context.

Restish takeaway: Restish docs should clearly mark deprecated flags, compatibility behavior, and recommended replacements. Do not bury migration guidance.

### 8. Supabase Docs

Source: [Supabase API References](https://supabase.com/docs/reference), [Supabase CLI Reference](https://supabase.com/docs/reference/cli/supabase-snippets-list)

Big-picture structure: Supabase combines product docs, client library references, Management API reference, and CLI reference under one docs system.

Page layout: The CLI reference opens with common use cases, additional links, global flags, then individual commands with usage and flags.

Tone and language: Direct and developer-friendly. It orients users around local development, migrations, deployment, project management, and type generation.

Examples and interactivity: Reference is structured and scannable. It puts global flags in one place so command pages can focus on command-specific behavior.

User journey: Supabase connects local CLI workflows with platform APIs, which is useful for users moving between terminal and service management.

Restish takeaway: Restish should have a global flags/reference page plus command-specific pages. Repeat global flags only when necessary; otherwise link back.

### 9. Cloudflare Developer Docs

Source: [Cloudflare API Shield overview](https://developers.cloudflare.com/api-shield/), [Wrangler commands](https://developers.cloudflare.com/workers/wrangler/commands/)

Big-picture structure: Cloudflare docs are product-specific but share a consistent developer-docs shell: overview, get started, concepts, configuration, reference, and resources.

Page layout: Pages include an "On this page" outline, short overview, command syntax, package-manager tabs, related resources, feedback controls, edit links, and last-updated dates.

Tone and language: Concise and operational. Cloudflare often gives the direct command first, then explains variations.

Examples and interactivity: Good use of package-manager tabs and command variants. The docs also expose Markdown/LLM-friendly versions, which is useful for automated consumption.

User journey: Strong for users who need to get from dashboard/platform concepts into command-line work.

Restish takeaway: Add page metadata discipline: last updated where the site supports it, edit links, related resources, and command variants only when they solve real user differences.

### 10. Docker Docs

Source: [Docker Reference documentation](https://docs.docker.com/reference/)

Big-picture structure: Docker separates guides/manuals from a reference hub that covers file formats, CLIs, APIs, drivers, and specs.

Page layout: The reference landing page is taxonomy-first. It helps readers choose Dockerfile, Compose file, Docker CLI, Compose CLI, Engine API, or related specs.

Tone and language: Functional and stable. It prioritizes discoverability across a large ecosystem.

Examples and interactivity: Docker's strength is not a single page pattern but a clean separation of reference surfaces.

User journey: Readers can enter by artifact type: file, command line, daemon, or API.

Restish takeaway: Restish docs should distinguish command behavior, config file behavior, OpenAPI/API behavior, plugin behavior, and internal design docs. Different artifacts need different page shapes.

### 11. Kubernetes / kubectl Docs

Source: [kubectl overview](https://kubernetes.io/docs/concepts/overview/kubectl/), [kubectl reference](https://kubernetes.io/docs/reference/kubectl/kubectl-cmds/)

Big-picture structure: Kubernetes keeps conceptual docs, task docs, tutorials, and reference docs distinct. kubectl docs include overview, command families, and generated command reference.

Page layout: kubectl reference pages use repeated sections: command examples, description, usage, flags, and inherited/global behavior. Examples often precede formal syntax.

Tone and language: Precise, operational, and explicit about declarative versus imperative workflows.

Examples and interactivity: The reference is example-rich but dense. Generated docs create consistency across a huge command surface.

User journey: Kubernetes guides users toward preferred workflows while still documenting every operational command.

Restish takeaway: For Restish command docs, lead each command section with 2-5 realistic examples, then usage and flags. Consider generated reference where CLI help can be the source of truth.

### 12. Terraform CLI Docs

Source: [Terraform CLI Documentation](https://developer.hashicorp.com/terraform/cli), [Terraform CLI overview](https://developer.hashicorp.com/terraform/cli/commands)

Big-picture structure: Terraform organizes CLI docs by workflow: initialize, provision, authenticate, write/modify code, inspect infrastructure, import, state, workspaces, plugins, config, automation, and alphabetical commands.

Page layout: Overview pages explain the command model and show raw CLI help output. Command pages include usage, flags, examples, and links to deeper workflow docs.

Tone and language: Calm, explanatory, and workflow-aware. Terraform distinguishes CLI mechanics from Terraform language docs.

Examples and interactivity: Hands-on tutorials are linked from reference pages, keeping reference concise while offering a learning path.

User journey: Terraform supports both "I am learning the workflow" and "I need exact command behavior."

Restish takeaway: Restish should separate request-building concepts from command syntax. Link reference pages to workflow guides such as "Make requests", "Authenticate", "Inspect responses", and "Automate with scripts".

### 13. GitHub CLI Manual

Source: [GitHub CLI manual](https://cli.github.com/manual/)

Big-picture structure: The `gh` manual is command-first, with installation/configuration up front and command groups beneath.

Page layout: Manual pages are terse: command description, usage, flags, examples, inherited flags, and see-also links.

Tone and language: Compact and terminal-native. It assumes readers are comfortable with CLI syntax.

Examples and interactivity: The manual mirrors command help, which keeps documentation and CLI behavior aligned.

User journey: Great for users who already know the command family and need exact syntax.

Restish takeaway: Mirror CLI help in reference pages where possible, but supplement with guides for cross-command workflows.

### 14. npm CLI Docs

Source: [npm CLI commands](https://docs.npmjs.com/cli/v11/commands), [npm command page](https://docs.npmjs.com/cli/v11/commands/npm/)

Big-picture structure: npm docs are versioned by CLI release and divide command reference, configuration, and usage docs.

Page layout: Command pages use synopsis, version, description, important notes, detailed conceptual sections, and see-also links. Version selectors are prominent.

Tone and language: Plain and occasionally informal, but reference-oriented.

Examples and interactivity: npm is strong on configuration details and "see also" links. It explicitly documents environment variables, config files, defaults, and modes.

User journey: The docs support users debugging configuration and behavior across versions.

Restish takeaway: Restish should document config precedence, environment variables, profiles, defaults, and version-sensitive behavior in one predictable reference area.

### 15. AWS CLI Command Reference

Source: [AWS CLI Command Reference](https://docs.aws.amazon.com/cli/latest/index.html)

Big-picture structure: AWS CLI docs are a massive generated command tree organized by service, then commands and subcommands.

Page layout: Pages are highly regular: description, synopsis, options, examples, output, and global options. The scale favors exhaustive lookup over learning.

Tone and language: Formal, generated, and terse.

Examples and interactivity: Examples are valuable, but the site can feel overwhelming because the service namespace is enormous.

User journey: Best for exact command lookup once users know the AWS service and operation.

Restish takeaway: Generated reference can scale, but Restish still needs curated guides and recipes so users are not dropped into an index with no journey.

### 16. Vercel CLI Docs

Source: [Vercel CLI Overview](https://vercel.com/docs/cli)

Big-picture structure: Vercel ties CLI docs to platform workflows: install, update, deploy, inspect logs, manage domains, and automate deployments.

Page layout: Overview pages start with what the CLI enables, then installation/update commands, then commands and platform links. Pages include freshness metadata and an "ask AI" affordance.

Tone and language: Clean, confident, and product-workflow focused.

Examples and interactivity: Copyable terminal commands are prominent. Cross-links route users to REST API docs when programmatic access is a better fit.

User journey: Vercel helps users decide between CLI, dashboard, and REST API paths.

Restish takeaway: Restish should help users decide when to use generic URL mode, API-aware commands from OpenAPI, plugin commands, or lower-level curl-like workflows.

## Best-Practice Sources

### Diataxis

Source: [Diataxis](https://diataxis.fr/), [How-to guides](https://diataxis.fr/how-to-guides/), [Tutorials](https://diataxis.fr/tutorials/)

Main idea: Documentation serves different user needs: learning, accomplishing, looking up facts, and understanding. Those needs map to tutorials, how-to guides, reference, and explanation.

Restish guidance:

- Do not mix tutorials, recipes, reference, and design explanations on the same page unless each section is clearly scoped.
- Getting-started pages should teach confidence through a complete path.
- Recipes should solve one task and avoid broad conceptual detours.
- Reference should be terse, complete, orderly, and optimized for lookup.
- Design docs should explain why the system is shaped a certain way, not replace user docs.

### Google Developer Documentation Style Guide

Sources: [Google developer documentation style guide](https://developers.google.com/style), [Prescriptive documentation](https://developers.google.com/style/prescriptive-documentation), [Accessible documentation](https://developers.google.com/style/accessibility), [Inclusive documentation](https://developers.google.com/style/inclusive-documentation)

Main ideas: Prioritize clarity and consistency, write for developer audiences, use prescriptive guidance when a recommended path exists, and make docs accessible and inclusive.

Restish guidance:

- Prefer direct language and active voice.
- Tell users what to do when Restish has a preferred path.
- Use "must", "can", and "might" precisely.
- Make headings descriptive and scannable.
- Use meaningful link text.
- Avoid screenshots of terminal text; use real text blocks.
- Keep examples culturally neutral and avoid unnecessary idioms.

### Microsoft Writing Style Guide

Sources: [Developer content](https://learn.microsoft.com/en-us/style-guide/developer-content/), [Writing step-by-step instructions](https://learn.microsoft.com/en-us/style-guide/procedures-instructions/writing-step-by-step-instructions), [Microsoft Learn style and voice quick start](https://learn.microsoft.com/en-us/contribute/content/style-quick-start)

Main ideas: Be warm, crisp, clear, and helpful. Developer docs rest heavily on reference and examples. Procedures should be easy to scan, imperative, consistent, and short when possible.

Restish guidance:

- Use imperative verbs in procedures.
- Keep procedures under seven steps where practical.
- Use parallel section headings and list structures.
- Include code examples that demonstrate the API or command element being described.
- Start by identifying the user's task and intent.

### Write the Docs

Sources: [Software documentation guide](https://www.writethedocs.org/guide/), [Documentation principles](https://www.writethedocs.org/guide/writing/docs-principles/), [Style guides](https://www.writethedocs.org/guide/writing/style-guides.html)

Main ideas: Good docs are participatory, example-rich, consistent, current, and maintained as part of the software lifecycle.

Restish guidance:

- Treat docs as part of the feature, not a final polish step.
- Use a project style guide to reduce cognitive load.
- Keep examples current and avoid version-specific details unless version matters.
- Give readers a way to report issues or discover related pages.

### The Good Docs Project

Sources: [The Good Docs Project](https://www.thegooddocsproject.dev/), [templates repository](https://github.com/thegooddocsproject/templates)

Main ideas: Templates help teams write consistently and reduce writer's block. Good docs should onboard users faster and reduce support load.

Restish guidance:

- Add page templates to `rsh-docs` for tutorials, recipes, reference pages, plugin docs, and design docs.
- Make the template prompts specific enough to produce a useful first draft.
- Encourage authors to delete unused template sections rather than keep empty scaffolding.

## Cross-Site Patterns Worth Copying

### Information Architecture

The strongest sites separate these layers:

- Start: install, first request, first success, next three things to learn.
- Guides: workflows that combine multiple commands or concepts.
- Recipes/how-tos: focused tasks with minimal detours.
- Reference: complete command, config, API, plugin, or schema facts.
- Concepts/explanation: mental models, tradeoffs, architecture, and design rationale.
- Troubleshooting: common failures, diagnostics, error interpretation.
- Changes: versioning, deprecations, migrations, compatibility.

Restish already has a similar model in the `rsh-docs` skill. The upgrade is to make the model more explicit and give each page type a stricter shape.

### Landing Pages

Good landing pages are curated routers, not passive indexes. Use groups such as:

- Start here
- Common workflows
- Popular reference
- Troubleshooting
- Plugin authors
- Maintainers

Each link should include one sentence that tells the reader why they would choose it.

### CLI Reference Pages

Strong CLI pages usually include:

- One-sentence purpose.
- Common examples first.
- Usage/synopsis.
- Arguments and flags.
- Global flags link.
- Input/output behavior.
- Environment variables and config interactions.
- Exit status or error behavior when relevant.
- Related commands and workflow guides.

For Restish, each command page should include at least one copyable command and representative output unless the command is destructive, environment-specific, or purely conceptual.

### API/HTTP Workflow Pages

Strong API pages usually include:

- What this workflow accomplishes.
- Requirements and authentication.
- Minimal request.
- Representative response.
- Important headers, content types, pagination, and errors.
- SDK/CLI/curl variants where useful.
- Troubleshooting and rate-limit/auth notes.
- Related pages.

For Restish, examples should show both generic URL requests and API-aware commands when that distinction matters.

### Design Docs

Developer-facing design docs should be explanation-oriented, but with enough structure to support decisions. A useful Restish design doc template:

- Title and status.
- Problem statement.
- Goals and non-goals.
- Current behavior and constraints.
- User-facing behavior.
- Proposed design.
- Alternatives considered.
- Compatibility and migration.
- Security, privacy, and failure modes.
- Testing and validation plan.
- Documentation impact.
- Open questions.
- Decision log or outcome.

Design docs should not substitute for user docs. If a design decision changes user behavior, add or update the user-facing page.

### Tone And Language

Best docs sound like a capable engineer helping another capable person:

- Direct but not curt.
- Friendly without filler.
- Precise about prerequisites and consequences.
- Honest about limitations, deprecations, and tradeoffs.
- Prescriptive when there is a recommended path.
- Neutral and accessible in examples.

Avoid vague phrases such as "simply", "obviously", "just", and "easy" when they minimize complexity.

### Examples

Example quality matters more than example volume.

- Prefer runnable examples using `https://api.rest.sh` when possible.
- Pair commands with output.
- Mark placeholders clearly.
- Explain why a placeholder is used when the public API cannot safely demonstrate the behavior.
- Show one happy path first, then variations.
- Include common failure output only when it helps users recover.
- Keep destructive workflows visibly separated and cautious.

### Interactivity And Docs-As-Product

Patterns worth considering for Restish over time:

- Copy buttons on command blocks.
- Language or shell tabs only when examples truly differ.
- Search that finds commands, flags, and config keys.
- "On this page" navigation for long reference.
- Last-updated metadata.
- Edit links.
- Feedback links.
- LLM-friendly Markdown output or `llms.txt` if the docs site grows.

## Proposed Upgrades To `rsh-docs`

### Add A Stronger Page-Type Contract

Add this rule: before writing, classify the page as tutorial, guide/how-to, recipe, reference, explanation/design, or troubleshooting. The page type determines structure, tone, and example density.

Suggested additions:

- Tutorial: learning-oriented, end-to-end, safe, complete, confidence-building.
- Guide/how-to: task-oriented, assumes a user goal, includes tradeoffs and decision points.
- Recipe: narrow task, short path, no conceptual sprawl.
- Reference: complete facts, terse descriptions, examples before exhaustive tables when useful.
- Explanation/design: why and how the system works, alternatives, constraints, consequences.
- Troubleshooting: symptoms, causes, diagnostics, fixes, prevention.

### Add Standard Templates

Add templates or checklists for:

- Getting started.
- Workflow guide.
- Recipe.
- Command reference.
- Config/reference page.
- Plugin operator page.
- Plugin author page.
- Troubleshooting page.
- Design doc.

### Tighten Reference Standards

For command reference, require:

- Purpose sentence.
- Common commands.
- Usage.
- Arguments.
- Flags.
- Output.
- Config/env interactions.
- Errors/exit behavior if relevant.
- Related pages.

For config reference, require:

- Scope and precedence.
- File location.
- Field names and types.
- Defaults.
- Examples.
- Related commands.

### Make User Journeys Explicit

When writing or editing a page, identify the reader's stage:

- First-time user.
- Daily CLI user.
- API integrator.
- Plugin operator.
- Plugin author.
- Maintainer.

Then route them to the next likely page at the end.

### Improve Example Discipline

Add to `rsh-docs`:

- Every operational page should include one copyable command.
- Prefer paired command/output examples.
- Use `api.rest.sh` canonical endpoints unless the example requires private infrastructure or destructive behavior.
- Keep placeholders intentional and explained.
- Avoid mixing live and placeholder hosts in one flow unless the contrast teaches something.

### Add Troubleshooting As A First-Class Page Type

Restish likely benefits from troubleshooting pages for:

- Authentication failures.
- OpenAPI loading and cache issues.
- Content negotiation surprises.
- Pagination behavior.
- Shorthand parsing.
- Plugin discovery and execution.
- Output formatting.

Use a repeated shape:

- Symptom.
- Likely cause.
- How to confirm.
- Fix.
- Related docs.

### Strengthen Design Doc Guidance

Add to `rsh-docs`:

- Significant features need a design doc before implementation.
- Design docs should preserve tradeoffs and decision history.
- User-visible behavior in a design doc must trigger user-doc updates.
- Design docs should include goals, non-goals, alternatives, compatibility, migration, tests, and docs impact.

### Add Maintenance Rules

Add a documentation maintenance checklist:

- Search for related pages before editing one page.
- Update landing pages when adding major docs.
- Check related links after moving or adding pages.
- Run the site build for meaningful changes.
- Grep for stale placeholders when adding live examples.
- Add TODO follow-ups when a page exposes a broader docs gap.

## Suggested `rsh-docs` Skill Patch Outline

This is not the patch itself, but the content structure to add to `.agents/skills/rsh-docs/SKILL.md`:

1. Documentation philosophy: docs serve user intent; classify page type first.
2. Page-type rules: tutorial, guide, recipe, reference, explanation/design, troubleshooting.
3. Voice: direct, warm, prescriptive, precise, accessible.
4. Example standard: copyable commands, output, placeholders policy, canonical endpoints.
5. Reference standard: command, config, API behavior, global flags, env/config precedence.
6. User journeys: first-time user, daily user, integrator, plugin operator, plugin author, maintainer.
7. Design docs: required sections and user-doc handoff.
8. Maintenance workflow: related pages, landing pages, TODOs, build, link checks.

## Short Checklist For Future Restish Docs

- What kind of page is this?
- Who is the reader?
- What does the reader want to accomplish or understand?
- Is the first useful command visible without hunting?
- Is there representative output?
- Are prerequisites and auth/config assumptions explicit?
- Are dangerous or private examples clearly marked?
- Is reference information complete enough for lookup?
- Are related pages linked?
- Does a design decision need a user-facing explanation?
- Can this page go stale, and what would keep it current?
