# structpages docs site

The site shell for https://jackielii.github.io/structpages/, built with [VitePress](https://vitepress.dev/).
The Markdown lives on `main` (`docs/*.md`, `README.md`, `PERFORMANCE.md`, `examples/README.md`); the Docs
Site workflow copies it into `docs/` (git-ignored here) and builds.

```bash
npm ci
npm run dev      # needs docs/ populated, see below
npm run build    # output in .vitepress/dist
```

To preview locally, copy the content in as the workflow does (`.github/workflows/docs.yml` on `main`).

`gsx` fences are highlighted with the gsx TextMate grammar from
[vscode-gsx](https://github.com/gsxhq/vscode-gsx), committed at `.vitepress/grammars/gsx.tmLanguage.json`.
Refresh it with `npm run sync-grammar` (expects `../gsxhq/vscode-gsx`).

After pushing to this branch, deploy with `gh workflow run "Docs Site"` from `main`.
