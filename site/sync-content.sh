#!/usr/bin/env bash
# Copies the Markdown the site renders from the repo into site/docs (git-ignored).
# The Docs Site workflow runs it before building; run it before `npm run dev`.
set -euo pipefail
site="$(cd "$(dirname "$0")" && pwd)"
repo="$(dirname "$site")"

rm -rf "$site/docs"
mkdir -p "$site/docs/reference" "$site/docs/examples"
cp "$repo"/docs/*.md "$site/docs/"
{
  printf -- '---\n'
  printf -- 'title: Examples\n'
  printf -- '---\n\n'
  cat "$repo/examples/README.md"
} > "$site/docs/examples/index.md"

# GNU and BSD sed differ on in-place editing; perl behaves the same on both.
# Rewrite cross-directory `../X` links (any sibling of docs/) to absolute
# GitHub URLs so they resolve on the rendered site.
find "$site/docs" -name '*.md' -print0 | xargs -0 perl -pi -e \
  's#\]\(\.\./([^)]+)\)#](https://github.com/jackielii/structpages/tree/main/$1)#g'
# The examples README links `./<name>/` relative to examples/; on the site
# those would resolve to /structpages/<name>, so point them at GitHub too.
perl -pi -e \
  's#\]\(\./([^)]+/?)\)#](https://github.com/jackielii/structpages/tree/main/examples/$1)#g' \
  "$site/docs/examples/index.md"

# The package reference, when gomarkdoc is installed (the workflow installs it).
if command -v gomarkdoc >/dev/null; then
  (cd "$repo" && gomarkdoc -o "$site/docs/reference/package.md" ./)
  # gomarkdoc anchors with <a name>, which VitePress never scrolls to. Move each
  # anchor onto the heading it precedes as a {#id} so #CurrentPage lands below
  # the navbar; the rest become <a id>.
  perl -0pi -e 's/<a name="([^"]+)"><\/a>\n(#+ [^\n]*)/$2 {#$1}/g; s/<a name="([^"]+)"><\/a>/<a id="$1"><\/a>/g' \
    "$site/docs/reference/package.md"
else
  echo "gomarkdoc not installed: skipping the package reference" >&2
  printf -- '# Package reference\n\nGenerated at deploy time.\n' > "$site/docs/reference/package.md"
fi
