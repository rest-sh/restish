---
title: Images in the Terminal
linkTitle: Images in the Terminal
weight: 70
description: Render image responses in capable terminals or save exact image bytes to files.
---

Restish can render `image/*` responses in terminals that support image
protocols, and can save the original bytes when you need a file.

## Render A Known Image

```bash
restish https://api.rest.sh/images/png -o image
restish https://api.rest.sh/images/jpeg -o image
```

## Negotiate An Image

```bash
restish -H 'Accept: image/png' https://api.rest.sh/image -o image
```

## Save The Bytes

```bash
restish https://api.rest.sh/images/png -o raw > image.png
restish https://api.rest.sh/images/jpeg -o raw > dragonfly.jpg
```

Use `-o raw` when exact bytes matter. Redirecting without `-o raw` may choose a
structured output default depending on content type and context.

## Terminal Support

Restish prefers native terminal image protocols where available and falls back
when the terminal cannot render images directly. If rendering fails, save the
file with `-o raw` and open it with an image viewer.

## Related Pages

- [Output](../output/)
- [Output Formats](/docs/reference/output-formats/)
- [Example API](/docs/reference/example-api/)
