import type {SidebarsConfig} from '@docusaurus/plugin-content-docs';

const sidebars: SidebarsConfig = {
  docs: [
    {
      type: 'category',
      label: 'Getting Started',
      collapsed: false,
      items: [
        'intro',
        'quick-start',
      ],
    },
    {
      type: 'category',
      label: 'Guides',
      collapsed: false,
      items: [
        'routing',
        'supported-flows',
        'templ',
        'htmx',
        'urlfor',
        'middleware',
        'advanced',
      ],
    },
    {
      type: 'category',
      label: 'Reference',
      collapsed: false,
      items: [
        'api',
        'reference/package',
      ],
    },
    {
      type: 'category',
      label: 'Examples',
      collapsed: false,
      items: [
        'examples/index',
      ],
    },
    {
      type: 'category',
      label: 'About',
      collapsed: true,
      items: [
        'about/performance',
      ],
    },
  ],
};

export default sidebars;
