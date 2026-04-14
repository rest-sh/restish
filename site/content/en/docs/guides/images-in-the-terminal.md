---
title: Images in the Terminal
linkTitle: Images in the Terminal
weight: 95
description: View image responses inline in supported terminals or save them unchanged.
---

Restish can render `image/*` responses inline when stdout is a terminal.

## Basic Usage

```bash
restish https://api.rest.sh/images/png
restish https://api.rest.sh/images/jpeg -o image
```

If stdout is a TTY, Restish can select the `image` formatter automatically for
image content.

## Rendering Behavior

Restish prefers native image display mechanisms when available, then falls back
to a Unicode half-block renderer. When stdout is not a TTY, image output falls
back to raw bytes.

## Save The Original Image

To preserve the response bytes exactly:

```bash
restish https://api.rest.sh/images/png > image.png
restish https://api.rest.sh/images/png -o raw > image.png
```

## Related Pages

- [Output Guide](/docs/guides/output/)
- [Output Formats Reference](/docs/reference/output-formats/)
- [Save a Response Unchanged Recipe](/docs/recipes/save-a-response-unchanged/)
