# Restish Docs Site

This directory contains the Restish v2 documentation site, built with Hugo and
the Docsy theme.

## Commands

If `hugo` is installed locally, install dependencies once:

```bash
npm ci
```

Run the site with:

```bash
npm run dev
```

Build the static site:

```bash
npm run build
```

## Notes

- Content lives under `content/en/docs/`.
- The theme is provided through Hugo Modules.
- Social preview images are generated into `static/images/social/` before
  Hugo runs. That directory is ignored because CI regenerates it for deploys.
- Generated output goes under `public/` and Hugo's resource cache; both are
  ignored.
