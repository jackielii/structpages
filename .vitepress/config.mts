import { defineConfig } from 'vitepress'
import gsxGrammar from './grammars/gsx.tmLanguage.json'
// The gsx grammar embeds Go, JavaScript and CSS. VitePress loads only the
// languages a fence names, so the embedded ones are registered explicitly.
import goLang from '@shikijs/langs/go'
import jsLang from '@shikijs/langs/javascript'
import cssLang from '@shikijs/langs/css'

const repo = 'https://github.com/jackielii/structpages'

export default defineConfig({
  title: 'structpages',
  description: 'Struct-based routing for Go web apps.',
  base: '/structpages/',
  cleanUrls: true,
  lastUpdated: false,
  // Serve docs/intro.md at /intro, as the Docusaurus site did.
  rewrites: {
    'docs/:page.md': ':page.md',
    'docs/:dir/:page.md': ':dir/:page.md',
  },
  srcExclude: ['README.md'],
  // quick-start links to the example server at http://localhost:8080.
  ignoreDeadLinks: 'localhostLinks',
  markdown: {
    // Highlight ```gsx fences with the gsx TextMate grammar from vscode-gsx
    // (refresh with `npm run sync-grammar`).
    languages: [
      ...goLang,
      ...jsLang,
      ...cssLang,
      { ...gsxGrammar, embeddedLangs: ['go', 'javascript', 'css'] },
    ],
  },
  themeConfig: {
    nav: [
      { text: 'Docs', link: '/intro' },
      { text: 'Examples', link: '/examples/' },
      { text: 'API Reference', link: '/reference/package' },
    ],
    sidebar: [
      {
        text: 'Getting Started',
        items: [
          { text: 'Introduction', link: '/intro' },
          { text: 'Quick Start', link: '/quick-start' },
          { text: 'Concepts', link: '/concepts' },
        ],
      },
      {
        text: 'Guides',
        items: [
          { text: 'Routing', link: '/routing' },
          { text: 'Page Response Patterns', link: '/supported-flows' },
          { text: 'gsx Patterns', link: '/gsx' },
          { text: 'Using templ', link: '/templ' },
          { text: 'HTMX Integration', link: '/htmx' },
          { text: 'URLFor & ID', link: '/urlfor' },
          { text: 'Error Handling', link: '/error-handling' },
          { text: 'Middleware', link: '/middleware' },
          { text: 'Advanced Features', link: '/advanced' },
        ],
      },
      {
        text: 'Reference',
        items: [
          { text: 'API Guide', link: '/api' },
          { text: 'Lint', link: '/lint' },
          { text: 'Package Reference', link: '/reference/package' },
        ],
      },
      { text: 'Examples', items: [{ text: 'Examples', link: '/examples/' }] },
      {
        text: 'About',
        collapsed: true,
        items: [{ text: 'Performance', link: '/about/performance' }],
      },
    ],
    socialLinks: [{ icon: 'github', link: repo }],
    search: { provider: 'local' },
    editLink: {
      // Markdown is copied in from main at build time (see the Docs Site
      // workflow), so map each page back to its source there. VitePress
      // serializes this function into the client: it must be self-contained.
      pattern: ({ filePath }) => {
        const base = 'https://github.com/jackielii/structpages'
        if (filePath === 'docs/about/performance.md') return `${base}/edit/main/PERFORMANCE.md`
        if (filePath === 'docs/examples/index.md') return `${base}/edit/main/examples/README.md`
        if (filePath.startsWith('docs/reference/')) return base
        return `${base}/edit/main/${filePath}`
      },
    },
    outline: [2, 3],
    footer: {
      message:
        '<a href="https://pkg.go.dev/github.com/jackielii/structpages">pkg.go.dev</a> · ' +
        '<a href="https://raw.githubusercontent.com/jackielii/structpages/main/llms.txt">llms.txt</a>',
      copyright: `MIT License. Copyright © ${new Date().getFullYear()} Jackie Li.`,
    },
  },
})
