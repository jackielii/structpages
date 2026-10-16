---
title: Concepts
slug: /concepts
sidebar_position: 3
---

# Concepts

structpages has its own canonical terms for its recurring patterns. Where a React / Next.js / React Router concept maps cleanly, it's noted as a cross-reference for knowledge transfer — but the structpages term is primary. Two guardrails: Go wins where Go owns the concept (`ServeHTTP` is a **handler method**, not a "server action"), and pure composition isn't named (a layout is just a **component** that takes **children** — there's no "layout route").

## Core nouns

| Term | What it is | Cross-ref |
|---|---|---|
| **page** | a route-tagged struct — a node in the route tree | Next/RR route/page |
| **page group** | a page with no render of its own (no `Page` or `ServeHTTP`), only child pages; served through its `/{$}` page | — (not a "layout route") |
| **component** | a standalone `component Foo(...)` — reusable, mount-independent, package-prefixed id | React component |
| **page component** | a method component `component (p Page) Foo(...)` — mount-aware, receiver in scope (incl. `Page`, `Content`). Used two ways: **composition** (called inside another page component) and **re-rendering** (returned alone as a partial) | React component (bound) |
| **children** | a `children gsx.Node` parameter rendered with `{ children }` | React children |
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
| **Props method** | `Props(...)` | loads data via DI → returns the props struct (or several values, each page component taking the ones it needs by type) |
| **handler method** | `ServeHTTP(...)` | imperative entry: mutate / redirect / serve JSON, or render a partial via `RenderComponent` — the Go `http.Handler` shape |
| **Middlewares method** | `Middlewares()` | declares middleware for the page + descendants |

(`Content` is not a framework concept — just a conventional page component name for a layout's main region; the matcher treats it like any other page component.)

The two render entries differ in flavor: the **Page method** renders declaratively (compose page components); the **handler method** renders imperatively (write the response, or hand a page component to `RenderComponent`). Both ultimately render through page components.

## Request lifecycle

**The request says what it wants. The Props method is the normal page flow; returning an error breaks it.**

For a rendering page: **route match → middleware → target selector** (reads the request into a `RenderTarget`: what it wants) **→ Props method** (loads data; may ask `target.Is(p.X)` to load only that region) **→ page component render**: the page component the target `Is`, else `Page`, its parameters filled by type from the props. Without `WithTargetSelector` every request wants `Page`; htmx apps opt in with `HTMXRenderTarget` (htmx 1/2) or `HTMXv4RenderTarget` (htmx 4), and then an HTMX request targeting a region's id renders that partial.

The Props method breaks the flow by returning an error: `RenderComponent(c, more...)` renders those components instead, `ErrSkipPageRender` means it already responded (a redirect), and any other error goes to the error handler. A handler method (`ServeHTTP`) bypasses this pipeline: it responds imperatively, optionally handing a page component to `RenderComponent`. The full flow is in [How a Request Is Rendered](./rendering.md).

## API helpers (literal — these are the public API)

[`RenderComponent`](./api.md#rendercomponent), [`RenderTarget`](./rendering.md#what-the-request-wants-the-target-selector), [`URLFor`](./urlfor.md#urlfor), [`ID` / `IDTarget`](./urlfor.md#id-and-idtarget) (in gsx, the [`url` / `id` / `target` filters](./gsx.md#register-the-filters)), [`Within`](./urlfor.md#active-navigation), [`Ref`](./urlfor.md#dynamic-references-with-ref), [`WithArgs`](./dependency-injection.md) (dependency injection / **args**).

## Loose comparisons (analogies, not structpages terms)

For readers arriving from React/Next — transfer aids, not structpages vocabulary.

| structpages | React/Next analogy | note |
|---|---|---|
| `/{$}` route of a page group | RR **index route** | nothing special — just the group's own page |
| **Page method** vs **handler method** | declarative `page` vs imperative **Route Handler / API route** | two ways to respond within one router — **not** "Page Router vs App Router" |
| **component** composition | Server Component composition | both render on the server |
