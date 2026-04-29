---
title: API Auth Inspect
linkTitle: API Auth Inspect
weight: 40
description: Inspect configured API auth without sending the target request.
---

`restish api auth inspect` answers the credential debugging question previously
handled by the removed top-level `auth-header` command. It resolves configured
auth without making the target API request, and it can inspect API-key headers
or a named operation credential as well as `Authorization`.

## Examples

```bash
restish api auth inspect example
restish -p staging api auth inspect example --raw-header Authorization
restish api auth inspect example --rsh-credential PartnerKey
```

## Notes

Use this to confirm profile auth before debugging a `401` or `403`. Human output
redacts sensitive values. Use `--raw-header Authorization` for scripts that
previously called `restish auth-header`.

## Related Pages

- [Commands](/docs/reference/commands/)
- [Authentication](/docs/guides/authentication/)
- [Profiles](/docs/reference/profiles/)
- [Global Flags](/docs/reference/global-flags/)
- [Troubleshooting](/docs/guides/troubleshooting/)
