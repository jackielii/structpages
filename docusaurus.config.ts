import {themes as prismThemes} from 'prism-react-renderer';
import type {Config} from '@docusaurus/types';
import type * as Preset from '@docusaurus/preset-classic';

const config: Config = {
  title: 'structpages',
  tagline: 'Struct-based routing for Go web apps',
  favicon: 'img/favicon.ico',

  url: 'https://jackielii.github.io',
  baseUrl: '/structpages/',

  organizationName: 'jackielii',
  projectName: 'structpages',
  trailingSlash: false,

  onBrokenLinks: 'warn',
  onBrokenMarkdownLinks: 'warn',

  i18n: {
    defaultLocale: 'en',
    locales: ['en'],
  },

  presets: [
    [
      'classic',
      {
        docs: {
          routeBasePath: '/',
          sidebarPath: './sidebars.ts',
          editUrl: 'https://github.com/jackielii/structpages/edit/main/',
        },
        blog: false,
        theme: {
          customCss: './src/css/custom.css',
        },
      } satisfies Preset.Options,
    ],
  ],

  plugins: [
    [
      require.resolve('@easyops-cn/docusaurus-search-local'),
      {
        hashed: true,
        indexBlog: false,
        docsRouteBasePath: '/',
        highlightSearchTermsOnTargetPage: true,
      },
    ],
  ],

  themeConfig: {
    image: 'img/social-card.png',
    colorMode: {
      defaultMode: 'light',
      respectPrefersColorScheme: true,
    },
    navbar: {
      title: 'structpages',
      logo: {
        alt: 'structpages logo',
        src: 'img/logo.svg',
      },
      items: [
        {to: '/intro', label: 'Docs', position: 'left'},
        {to: '/examples/', label: 'Examples', position: 'left'},
        {to: '/reference/package', label: 'API Reference', position: 'left'},
        {
          href: 'https://github.com/jackielii/structpages',
          label: 'GitHub',
          position: 'right',
        },
      ],
    },
    footer: {
      style: 'dark',
      links: [
        {
          title: 'Docs',
          items: [
            {label: 'Introduction', to: '/intro'},
            {label: 'Quick Start', to: '/quick-start'},
            {label: 'API Reference', to: '/reference/package'},
          ],
        },
        {
          title: 'Resources',
          items: [
            {label: 'pkg.go.dev', href: 'https://pkg.go.dev/github.com/jackielii/structpages'},
            {label: 'GitHub', href: 'https://github.com/jackielii/structpages'},
            {label: 'llms.txt', href: 'https://raw.githubusercontent.com/jackielii/structpages/main/llms.txt'},
          ],
        },
      ],
      copyright: `MIT License. Copyright © ${new Date().getFullYear()} Jackie Li.`,
    },
    prism: {
      theme: prismThemes.github,
      darkTheme: prismThemes.dracula,
      additionalLanguages: ['go', 'bash', 'yaml'],
    },
  } satisfies Preset.ThemeConfig,
};

export default config;
