import { h } from 'vue'
import DefaultTheme from 'vitepress/theme'
import './custom.css'

export default {
  extends: DefaultTheme,
  // The release status sits above the hero title on the home page.
  Layout: () =>
    h(DefaultTheme.Layout, null, {
      'home-hero-info-before': () => h('p', { class: 'status-badge' }, 'Beta — battle-tested in production'),
    }),
}
