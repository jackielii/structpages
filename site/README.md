# structpages docs site

The site for https://jackielii.github.io/structpages/, built with [VitePress](https://vitepress.dev/).
The Markdown lives elsewhere in the repo (`docs/*.md`, `examples/README.md`); `sync-content.sh` copies it
into `site/docs/` (git-ignored) along with a generated package reference.

```bash
cd site
npm ci
./sync-content.sh
npm run dev      # or: npm run build (output in .vitepress/dist)
```

The Docs Site workflow (`.github/workflows/docs.yml`) does the same and deploys on every push to `main`
that touches the docs, the examples README, the Go API or `site/`.

`site/go.mod` makes this directory its own module, so none of it lands in the structpages module zip.

`gsx` fences are highlighted with the gsx TextMate grammar from
[vscode-gsx](https://github.com/gsxhq/vscode-gsx), committed at `.vitepress/grammars/gsx.tmLanguage.json`.
Refresh it with `npm run sync-grammar` (expects `../../gsxhq/vscode-gsx`).
