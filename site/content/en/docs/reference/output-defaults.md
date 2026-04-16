---
title: Output Defaults
linkTitle: Output Defaults
weight: 28
description: Reference for Restish's default output choices on TTYs, redirects, filters, and raw output.
---

Restish changes its default output behavior based on whether stdout is a TTY and
whether the output is still the original wire payload or a normalized value.

## Main Rule

- on a terminal, structured output defaults to `readable`
- off a terminal, structured output defaults to JSON
- raw bytes are preserved when you explicitly ask for `-o raw`, or when the
  response is still being passed through as raw bytes

## Why The Defaults Exist

Interactive use usually needs:

- status and headers
- human-readable formatting
- context for exploration

Redirected output usually needs:

- one stable machine-friendly document
- fewer surprises in scripts

## Filtering Changes The Contract

Once you filter a response, you are no longer asking for the original wire
payload. You are asking for a transformed value.

That is why filtered output defaults to structured rendering, not raw bytes.

Use `-r` only when the filtered result is a scalar or a scalar array and you
want shell-friendly output.

## Document vs Record Output

Think in two families:

- document output: `readable`, `json`, `yaml`
- record output: `ndjson`

Document formats aim to preserve one coherent result. Record formats emit one
item or event at a time.

## Common Explicit Choices

- `-o json` for one JSON document
- `-o yaml` for one YAML document
- `-o ndjson` for one JSON value per line
- `-o raw` for exact body bytes
- `-o table` for arrays of objects

## Related Pages

- [Output Guide](/docs/guides/output/)
- [Output Formats](../output-formats/)
- [Global Flags](../global-flags/)
