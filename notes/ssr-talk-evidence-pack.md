# Evidence Pack: Go SSR (structpages + templ + HTMX) vs React SPA + Go backend

> Working material for blog post + talk. Audience: a team that believes "robust, fast AI
> agentic coding" for the HIS project means AI-generated React frontend + Go backend.
> Research: 104-agent deep-research run (22 sources fetched, 109 claims extracted,
> 25 adversarially verified: 21 confirmed / 4 killed) + exploration of one-learning and
> his-project + git-history forensics. Date: 2026-06-11.

---

## 0. The argument in one paragraph

The team's question is not "React vs HTMX" in the abstract — it is *which stack lets AI
agents complete the HIS project fastest and most robustly*. Agentic coding quality is the
product of three factors: **generation fluency × verification loop × blast radius**. React
wins only the first, and only partially. The Go SSR stack wins the other two: a
deterministic, seconds-fast, compile-time verification loop over the entire stack (templ +
go build + structpages-lint + sqlc), and roughly 60% less code per feature with no
unverifiable JSON seam where agent hallucinations hide. We don't need to argue this from
theory — we already ran the experiment inside his-project itself: the doctor portal was
built both ways, and the React version was stripped out in June 2026.

---

## 1. First-party evidence (the strongest material — it's our own product)

### 1.1 The doctor portal A/B test (his-project)

The same clinical portal, same product, same team, built both ways. Commits:

- `aeda3afe` — `refactor(doctor): strip the React SPA + /api/doctor JSON layer`
  (2026-06-10): **280 files, −38,761 lines**. Removed `modules/doctor/fe` (React+Vite,
  16,298 lines incl. bun.lock) and `modules/doctor-api` (Gin JSON API: handlers, specs,
  tests — 21,847 lines). `go.mod` dropped gin-gonic **plus ~20 transitive deps**.
- `3e7ae88a` — `feat(doctor/consult): like-for-like port of the React portal's clinical
  features` (same day): **56 files, +5,716 lines**, each section with unit + RLS
  integration tests.

Honest like-for-like comparison (both sides include tests):

| | React SPA + JSON API | SSR (structpages + templ) |
|---|---|---|
| Total hand-written code for the doctor surface | **38,145 lines** (16,298 fe + 21,847 api) | **15,225 lines** (10,820 .go + 4,405 .templ, generated code excluded) |
| Reduction | | **~60%** |
| Extra dependencies | gin + ~20 transitive, React/Vite/bun toolchain | none (shares the app's stack) |
| The JSON seam | 21.8k lines existing *only because* of the split | does not exist |

Caveats to state in the post: the −38.7k includes specs/docs/lockfile; quote the 60%
figure from the table, not the raw diff. This independently reproduces the Contexte
case study's 67% (below) — two data points, one of them ours.

The port also shows the AI-agent angle: 89 of 840 commits carry explicit
`Co-Authored-By: Claude` trailers (an undercount — squashed PRs hide trailers), the repo
carries `CLAUDE.md` + 120+ ADRs, and structpages ships a Claude Code skill. The
like-for-like port of an entire clinical feature set landed in a day. **TODO (Jackie):
confirm how much of the port was agent-driven so the claim can be made precisely.**

### 1.2 Longevity evidence (one-learning)

- ~2.5 years, 1,867 commits, production: 4 entity types, dashboards, bulk import, email
  workflows, LLM embeddings, RBAC, analytics.
- ~279k hand-written LoC (207k Go + 72k templ); **~6,700 lines of client JS total**.
- 56 direct Go deps; **34 npm packages total** (vs 200–400+ typical for a Next.js app).
- No framework churn in 2.5 years; full observability (Prometheus, OTel server+browser).
- Single ~15 MB Docker image, health probes, migrations on startup.

### 1.3 Stack thinness (his-project)

- Frontend `package.json`: **5 runtime deps** (tailwind, vite, htmx, alpine, a font).
- 3-stage Dockerfile → distroless image, assets embedded via `go:embed`; full build <30s.
- `cmd/latency-bench`: in-repo p50/p95/p99 latency tool with budgets — use these numbers
  for performance claims instead of Lighthouse (decision: **no Lighthouse** — gameable,
  high-variance, invites "we can match that").

---

## 2. Verified web evidence (confidence per deep-research adversarial verification)

### 2.1 The hypermedia canon (HIGH confidence on attribution; advocacy, pair with data)

Carson Gross, *When to Use Hypermedia* (https://htmx.org/essays/when-to-use-hypermedia/):
- "Hypermedia is often significantly less complex than an SPA approach would be for many problems."
- "Hypermedia allows your application API to be much more aggressively refactored and optimized."
  → maps directly to structpages Props: no versioned JSON contract; Props + templ change
  in one commit; URLFor + lint make refactors compile-safe. Fielding's line backs it:
  "a uniform interface degrades efficiency."
- "Hypermedia takes pressure off adopting a particular server technology."
  → the bridge to Go: choosing React gravitationally pulls the org toward Node/Next/TS;
  hypermedia frees the backend choice — the "HOWL stack" argument.
- Good fit: text & images, CRUD/forms-into-database, updates in well-defined blocks, deep
  links + first-render performance. **A health information system is the archetype.**
- *Hypermedia-Driven Applications* essay (HIGH): MPA simplicity + SPA-comparable UX via
  declarative HTML attributes. Phrase precisely: htmx is ~14kB of JS, so say "no
  JavaScript *application code*," not "no JavaScript."

### 2.2 Case study: Contexte React→htmx port (MEDIUM — single self-reported study)

https://htmx.org/essays/a-real-world-react-to-htmx-port/ (David Guillot, DjangoCon 2022):
**67% codebase reduction (21,500→7,200 LoC), 255→9 JS deps (−96%), 50–60% faster
time-to-interactive, 46% lower memory.** Cite as a favorable case study, not a
generalizable statistic; note the app was text-oriented (hypermedia's best case) and that
Contexte kept Vue for its real-time collaborative editor. Our doctor-portal 60% is the
corroborating second data point.

### 2.3 Grug brain (MEDIUM — rhetoric, use as cultural framing)

https://grugbrain.dev/ — "two complexity demon spirit lairs": the SPA+API split doubles
complexity centers "even when website just need put form into database." Disclose the
author concentration: htmx essays + grugbrain are substantially one author (Gross) with a
stake; a sharp audience will spot it if we hide it.

### 2.4 DHH / No Build (HIGH as shipped existence proof, NOT industry consensus)

- Campfire (ONCE) shipped commercially (~$250k first week) with **no front-end build
  step**; Rails 8 (Nov 2024) made no-build the default.
- "The state of the art is no longer finding more sophisticated ways to build JavaScript
  or CSS. It's not to build at all."
- ⚠️ The claim that browser support made bundling universally skippable was **REFUTED
  (0-3)** in verification. Argue from the shipped product, never from the universality
  premise. (We use Vite for Tailwind anyway — our claim is *thin* build, not *no* build.)

### 2.5 Go vs Node server-side HTML throughput (HIGH, but frame per-implementation)

TechEmpower Round 23 Fortunes (DB query + HTML templating — the SSR-shaped benchmark):
- Fastest Go (fasthttp-prefork): 959,399 rps ≈ **3.4×** fastest raw Node (283,445) and
  **~12×** Express+Postgres (78,136).
- ⚠️ Honest framing required: idiomatic gin (110,596) and chi (121,198) trail
  fastify-postgres (265,826); Go's headline wins are prefork/fasthttp entries. Say "the
  Go ceiling is far higher and the common Node path (Express) is far slower," not "any Go
  beats any Node." Round 23 (Feb 2025) is the final round (project sunset Mar 2026).
- The stronger performance story for the talk is architectural anyway: **TTFB /
  time-to-content**. An SSR response's first byte contains the content (vs SPA's
  shell→bundle→API waterfall), and Go+templ renders in µs–ms with flat memory, so good
  TTFB is the *default* — whereas Node/Next SSR is expensive enough that it needs
  streaming, PPR, and edge caching as mitigations. React moving server-side (RSC)
  concedes thesis #1 (SSR is right); once rendering is on the server, the runtime doing
  it matters — thesis #2 (Go beats Node at serving).

### 2.6 RSC complexity (corroborated)

Even hypermedia critics' side concedes: RSC/Qwik are "remarkable engineering achievements,
but… quite complicated" (Gross); independently, Josh Comeau (pro-React) calls the RSC
paradigm "significantly more complex… pretty confusing." ⚠️ Coverage gap: the broader
"Next.js backlash 2024-2026 / OpenNext / Vercel lock-in" angle produced **no surviving
verified claims** — do not present it as established without a dedicated research pass.
(Leads that were fetched but unverified: flightcontrol.dev self-hosting post, LogRocket
OpenNext post, several HN threads.)

### 2.7 The "AI generates React better" claim — what the evidence actually says

For us:
- **Popularity ≠ AI proficiency** (arXiv 2509.11132, preprint, Python libs): concordance
  between GitHub stars and LLM code quality is 0.574, Cohen's d = 0.15 (significant but
  *very weak*); functionally similar libraries differ by up to **84%** in generated-code
  quality. ⚠️ Never tested React or HTMX — present as an inference, and as "weak
  correlation," not "no correlation."
- **Go the language is near-parity for LLMs** (ICLR 2026, arXiv 2509.23261,
  peer-reviewed): DeepSeek-V3 Pass@1 — Python 79.8%, JavaScript 76.9%, **Go 76.8%**. The
  scary "popular languages beat unpopular by 29–45pp" group stat is driven by Erlang/
  Racket, not Go.

Against us (MUST address, do not cherry-pick the same paper):
- The **same ICLR paper's framework-tier result**: in agentic full-stack experiments
  (Cursor/CodeBuddy + Claude-4-Sonnet, Copilot + GPT-5), React+Express failed 1/17 tasks
  while the "Lightweight Go" stack failed **7/17**. Mitigations to state honestly:
  (a) the Go stack tested was **Preact+Gin+GORM — a Go-API-plus-SPA split, not
  Go+templ+HTMX**, so it tested the architecture we're arguing *against* with a Go
  backend; (b) bare agents, no framework docs in context; (c) mid-2025 models.
- The load-bearing rebuttal — **does a skill/docs-in-context close the niche-framework
  gap?** — is currently *unverified by any published study*. We have anecdotal proof
  (his-project itself, built agent-first with CLAUDE.md + ADRs + the structpages skill)
  but should consider producing the evidence: re-run a handful of the ICLR-style tasks
  on Go+templ+HTMX+structpages with the skill loaded vs without. Even 5 tasks would make
  the blog post's centerpiece original research nobody else has.

### 2.8 Killed claims — lines of argument to AVOID (each refuted 0-3)

1. "Import maps/browser support make bundling universally skippable" (DHH premise).
2. "htmx can replicate SPA UX like persistent video across navigations."
3. "HTMX+templ measurably boosts developer productivity" (the konfigthis source failed
   verification — don't cite it in either direction).
4. Also avoid the strawman "HTMX is only for simple use cases" as *the* opposing
   argument — that framing failed verification too; the real opposing argument is the
   ICLR framework-tier result above.

---

## 3. Honest scope boundary (the steelman section — include it, it buys credibility)

From the advocates themselves (verified): hypermedia is a poor fit for
- UIs with many dynamic interdependencies not knowable at server render time
  (spreadsheets; GitHub's stale issue-count tab is a named failure mode);
- offline-first apps;
- very high-frequency state updates (games, live cursors);
- teams without buy-in (Gross lists this as a *legitimate* reason not to adopt — our
  answer: the buy-in cost at our org is already paid and amortized across two production
  codebases).

Concede Figma/Sheets/collaborative-editor territory explicitly (Contexte kept Vue for
exactly that). The HIS project has none of these characteristics — it is forms, records,
schedules, and dashboards: the archetypal hypermedia-shaped product.

---

## 3.5 The composition-framework thesis (Jackie's framing — make it a pillar)

Most recurring objections to the hypermedia approach — handler sprawl for every partial,
template-fragment spaghetti, stringly-typed `hx-target`s, "it doesn't compose" — are not
objections to hypermedia. They are objections to doing hypermedia **without a server-side
composition framework**. Rails (Turbo) and Django people rarely raise them, because their
framework *is* the composition layer. Go's stdlib gives a great mux and nothing above it,
so teams wire HTMX to a pile of handlers and fragments, hate it, and blame HTMX.

structpages is the missing layer: composition is struct embedding (page trees, layouts,
shared design-system templ components); a partial is just another templ method on the
page — on an HTMX request the RenderTarget selector matches `hx-target` to the component
method and renders only that (same route, same Props, no second handler); `ID`/`IDTarget`
and `URLFor` make targets and URLs compile-time-safe. Terminology note (Jackie's call):
say **"the hypermedia approach"**, not "just HTML" or bare "SSR" — the claim is the
architectural model (HTML as the engine of application state), not merely where rendering
happens.

## 4. Proposed narrative arc

**Blog post: "We built the same portal twice" (working title)**
1. Cold open: the diff. `−38,761 / +5,716`, same features, same day, our own product.
2. Reframe the debate: not React vs HTMX — *what makes agentic coding succeed*
   (generation × verification × blast radius).
3. The verification loop: walk one feature through Props → templ → lint → build; contrast
   with the JSON seam (show a real field-name drift class of bug that the compiler
   catches in our stack and nothing catches in the split stack).
4. The evidence, ours + external: doctor portal 60% / Contexte 67% / one-learning 2.5yr.
5. Performance as architecture: TTFB contains content; Go renders in µs; React's own
   server turn (RSC + its complexity discourse) concedes the direction.
6. Steelman: ICLR 7/17 + the not-good-fit list + author concentration; our mitigation
   (skill + CLAUDE.md + ADRs) and — ideally — our replication experiment.
7. Close: simplicity wins *because* it compounds — for humans and agents alike.

**Talk: same arc, lead slide = the diff stat; demo = agent adds a feature live with the
skill loaded; closing slide = "the stack the agent can verify is the stack you can trust."**

---

## 5. Open follow-ups

- [ ] Confirm AI-agent share of the doctor-portal port (for precise claims).
- [ ] Optional but high-value: run the skill-vs-no-skill agentic experiment (§2.7).
- [ ] Dedicated research pass on Next.js/Vercel/OpenNext discourse if we want §2.6
      expanded (currently unverified).
- [ ] Pull real p50/p95/p99 numbers from `cmd/latency-bench` for the performance section.

## Source list (verified)

- https://htmx.org/essays/when-to-use-hypermedia/
- https://htmx.org/essays/hypermedia-driven-applications/
- https://htmx.org/essays/a-real-world-react-to-htmx-port/
- https://htmx.org/essays/a-response-to-rich-harris/
- https://grugbrain.dev/
- https://world.hey.com/dhh/you-can-t-get-faster-than-no-build-7a44131c
- https://www.techempower.com/benchmarks/ (Round 23 Fortunes)
- https://arxiv.org/pdf/2509.11132 (popularity vs AI proficiency, preprint)
- https://arxiv.org/pdf/2509.23261 (ICLR 2026, language + framework tiers — cite both results)
- Unverified leads for §2.6: flightcontrol.dev self-host-nextjs, LogRocket OpenNext,
  HN 40828610 / 38018217 / 43881035 / 43672449
