# User Intent

- Start from the current prompt and the current repository state.
- Prefer direct execution for small requests.
- For larger requests, make a clear plan before mutating files.
- Keep this file for stable user facts, constraints, and preferences that remain useful across runs.
- orch may append a hidden machine-managed preference block at the end of this file.
- Do not copy `PRODUCT.md` content into this file.
