---
title: Post JSON From a File
linkTitle: Post JSON From a File
weight: 25
description: Send a JSON request body to an API from a file instead of inline shorthand.
---

If the body already exists on disk, pipe it to Restish:

```bash
cat payload.json | restish post https://api.rest.sh
```

If you want to keep the command explicit about JSON, set the content type too:

```bash
cat payload.json | restish post -c json https://api.rest.sh
```

This is a better fit than shorthand when the body is large or already produced
by another tool.
