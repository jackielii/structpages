---
title: Concepts
slug: /concepts
sidebar_position: 3
---

# Concepts & Vocabulary

structpages has its own canonical terms for its recurring patterns. Where a React / Next.js / React Router concept maps cleanly, it's noted as a cross-reference for knowledge transfer — but the structpages term is primary. Two guardrails: Go wins where Go owns the concept (`ServeHTTP` is a **handler method**, not a "server action"), and pure composition isn't named (a layout is just a **component** that takes **children** — there's no "layout route").

## Core nouns

| Term | What it is | Cross-ref |
|---|---|---|
| **page** | a route-tagged struct — a node in the route tree | Next/RR route/page |
| **page group** | a page with no render of its own (no `Page` or `ServeHTTP`), only child pages; served through its `/{$}` page | — (not a "layout route") |
| **component** | a standalone `templ Foo()` block — reusable, mount-independent, package-prefixed id | React component |
| **page component** | a `templ (p Page) Foo()` method — mount-aware, receiver in scope (incl. `Page`, `Content`). Used two ways: **composition** (called inside another page component) and **re-rendering** (returned alone as a partial) | React component (bound) |
| **children** | templ `{ children... }` composition | React children |
| **partial** | a page component returned on its own as an HTMX response to re-render just that region — a *role* a page component plays, not a distinct kind | HTMX |

## The props cluster

| Term | What it is | Cross-ref |
|---|---|---|
| **Props method** | the `Props(...)` method that loads data via DI | *like RR `loader` / Next `getServerSideProps`* |
| **props struct** | the named struct type the Props method returns and page components accept | *like a React props type* |
| **props** | a value of the props struct, in flight into a page component | React props (the value) |

The chain reads: the **Props method** returns a **props struct**; that **props** value is handed to a **page component**.

## Methods on a page

| Term | Method | Job |
|---|---|---|
| **Page method** | `Page(props)` | the main render entry — a page component that composes the full page (layout + content) |
| **Props method** | `Props(...)` | loads data via DI → returns the props struct |
| **handler method** | `ServeHTTP(...)` | imperative entry: mutate / redirect / serve JSON, or render a partial via `RenderComponent` — the Go `http.Handler` shape |
| **Middlewares method** | `Middlewares()` | declares middleware for the page + descendants |

(`Content` is not a framework concept — just a conventional page component name for a layout's main region; the matcher treats it like any other page component.)

The two render entries differ in flavor: the **Page method** renders declaratively (compose page components); the **handler method** renders imperatively (write the response, or hand a page component to `RenderComponent`). Both ultimately render through page components.

## Request lifecycle

For a rendering page: **route match → Props method** (with `RenderTarget` injected to pick the region) **→ page component render** — `Page` for full loads, a partial for HTMX requests targeting that region's id. A handler method (`ServeHTTP`) bypasses this pipeline: it responds imperatively, optionally handing a page component to `RenderComponent`.

## API helpers (literal — these are the public API)

`RenderComponent`, `RenderTarget`, `URLFor`, `ID` / `IDTarget`, `Ref`, `WithArgs` (dependency injection / **args**).

## Loose comparisons (analogies, not structpages terms)

For readers arriving from React/Next — transfer aids, not structpages vocabulary.

| structpages | React/Next analogy | note |
|---|---|---|
| `/{$}` route of a page group | RR **index route** | nothing special — just the group's own page |
| **Page method** vs **handler method** | declarative `page` vs imperative **Route Handler / API route** | two ways to respond within one router — **not** "Page Router vs App Router" |
| **component** composition | Server Component composition | both render on the server |
