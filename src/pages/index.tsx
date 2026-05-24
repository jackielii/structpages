import React from 'react';
import Link from '@docusaurus/Link';
import Layout from '@theme/Layout';
import CodeBlock from '@theme/CodeBlock';
import styles from './index.module.css';

const QUICK_START_SNIPPET = `package main

import (
    "log"
    "net/http"
    "github.com/jackielii/structpages"
)

type index struct {
    product \`route:"/product Product"\`
    team    \`route:"/team Team"\`
    contact \`route:"/contact Contact"\`
}

templ (index) Page() {
    <html>
        <body>
            <h1>Welcome</h1>
        </body>
    </html>
}

func main() {
    mux := http.NewServeMux()
    if _, err := structpages.Mount(mux, index{}, "/", "Home"); err != nil {
        log.Fatal(err)
    }
    log.Println("Listening on :8080")
    http.ListenAndServe(":8080", mux)
}`;

const FEATURES = [
  {
    title: 'Struct-based routing',
    body: 'Define routes as Go struct fields with tags. Nesting structs creates nested route hierarchies. No magic registration calls.',
  },
  {
    title: 'Templ + HTMX',
    body: 'First-class Templ support. HTMX-aware partial rendering via the default HTMXRenderTarget — return the right component for hx-target without manual dispatch.',
  },
  {
    title: 'Type-safe URLs',
    body: 'URLFor generates URLs from struct references, ID/IDTarget generates matching HTML ids and CSS selectors. The structpages-lint analyzer catches mismatches at build time.',
  },
];

export default function Home(): JSX.Element {
  return (
    <Layout
      title="structpages"
      description="Struct-based routing for Go web apps. Built-in Templ, HTMX, and type-safe URLs."
    >
      <header className={styles.hero}>
        <div className="container">
          <h1 className={styles.heroTitle}>structpages</h1>
          <p className={styles.heroTagline}>
            Struct-based routing for Go web apps.
          </p>
          <p className={styles.heroSubtitle}>
            Built on <code>http.ServeMux</code>. First-class Templ and HTMX. Type-safe URLs.
          </p>
          <p className={styles.alphaBadge}>Alpha — APIs may change</p>
          <div className={styles.heroButtons}>
            <Link className="button button--primary button--lg" to="/intro">
              Get Started
            </Link>
            <Link
              className="button button--secondary button--lg"
              href="https://github.com/jackielii/structpages"
            >
              GitHub
            </Link>
          </div>
        </div>
      </header>
      <main>
        <section className={styles.features}>
          <div className="container">
            <div className="row">
              {FEATURES.map((feature) => (
                <div key={feature.title} className="col col--4">
                  <h3>{feature.title}</h3>
                  <p>{feature.body}</p>
                </div>
              ))}
            </div>
          </div>
        </section>
        <section className={styles.snippet}>
          <div className="container">
            <h2>Quick look</h2>
            <CodeBlock language="go">{QUICK_START_SNIPPET}</CodeBlock>
            <p>
              <Link to="/quick-start">See the full walkthrough →</Link>
            </p>
          </div>
        </section>
        <section className={styles.links}>
          <div className="container">
            <h2>Resources</h2>
            <ul>
              <li><a href="https://pkg.go.dev/github.com/jackielii/structpages">pkg.go.dev reference</a></li>
              <li>Claude Code plugin: <code>/plugin marketplace add jackielii/structpages</code></li>
              <li><a href="https://raw.githubusercontent.com/jackielii/structpages/main/llms.txt">llms.txt</a></li>
            </ul>
          </div>
        </section>
      </main>
    </Layout>
  );
}
