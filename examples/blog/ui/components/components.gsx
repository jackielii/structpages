// Package components exports the framework-agnostic UI primitives shared by
// every feature package: buttons, form fields, alerts, cards, pagination,
// and the styled error component used by the global error handler.
//
// Each function is a standalone gsx component (not a method on a page
// struct). That means they can be addressed by HTMX as targets via their
// kebab-cased name — e.g. hx-target="#pagination" matches Pagination.
package components

type AlertKind string

const (
	AlertInfo    AlertKind = "info"
	AlertError   AlertKind = "error"
	AlertSuccess AlertKind = "success"
)

func alertClasses(kind AlertKind) string {
	switch kind {
	case AlertError:
		return "border-red-200 bg-red-50 text-red-800"
	case AlertSuccess:
		return "border-emerald-200 bg-emerald-50 text-emerald-800"
	default:
		return "border-slate-200 bg-slate-50 text-slate-800"
	}
}

component Alert(kind AlertKind, msg string) {
	{ if msg != "" {
		<div class={ "rounded border px-3 py-2 text-sm", alertClasses(kind) }>
			{ msg }
		</div>
	} }
}

component Card(title string) {
	<section class="rounded-lg border bg-white p-5 shadow-sm">
		{ if title != "" {
			<h2 class="mb-3 text-base font-semibold text-slate-900">
				{ title }
			</h2>
		} }
		{ children }
	</section>
}

// Button takes only its label; any extra attributes (type, hx-*, etc.) fall
// through to the root <button> automatically — no explicit attrs param.
component Button(label string) {
	<button
		class="inline-flex items-center rounded bg-slate-900 px-3 py-1.5 text-sm font-medium text-white hover:bg-slate-700"
	>
		{ label }
	</button>
}

component Input(name, label, value, errMsg string) {
	<label class="block text-sm">
		<span class="mb-1 block font-medium text-slate-700">{ label }</span>
		<input
			name={name}
			value={value}
			class={
				"w-full rounded border px-2 py-1.5 text-sm", "border-slate-300": errMsg == "", "border-red-400": errMsg != ""
			}
		/>
		{ if errMsg != "" {
			<span class="mt-1 block text-xs text-red-600">{ errMsg }</span>
		} }
	</label>
}

component Textarea(name, label, value, errMsg string) {
	<label class="block text-sm">
		<span class="mb-1 block font-medium text-slate-700">{ label }</span>
		<textarea
			name={name}
			rows="6"
			class={
				"w-full rounded border px-2 py-1.5 text-sm", "border-slate-300": errMsg == "", "border-red-400": errMsg != ""
			}
		>{ value }</textarea>
		{ if errMsg != "" {
			<span class="mt-1 block text-xs text-red-600">{ errMsg }</span>
		} }
	</label>
}

// Pagination is rendered as a standalone function component so HTMX requests
// with HX-Target: #pagination resolve here regardless of which page hosts it.
component Pagination(p PageNav) {
	<nav id="pagination" class="flex items-center justify-between text-sm">
		<span class="text-slate-500">{ p.Range() }</span>
		<div class="flex gap-2">
			{ if p.HasPrev() {
				<a
					class="rounded border px-2 py-1 hover:bg-slate-50"
					href={p.URL(p.Page - 1)}
				>
					← Prev
				</a>
			} else {
				<span class="rounded border px-2 py-1 text-slate-300">
					← Prev
				</span>
			} }
			<span class="px-2 py-1 text-slate-600">
				Page { p.Page } of { p.Pages() }
			</span>
			{ if p.HasNext() {
				<a
					class="rounded border px-2 py-1 hover:bg-slate-50"
					href={p.URL(p.Page + 1)}
				>
					Next →
				</a>
			} else {
				<span class="rounded border px-2 py-1 text-slate-300">
					Next →
				</span>
			} }
		</div>
	</nav>
}

// ErrorPage is the full document rendered by main.errorHandler for non-HTMX
// errors. ErrorBlock is the partial used for HTMX requests.
component ErrorPage(status int, msg string) {
	<!DOCTYPE html>
	<html lang="en">
		<head>
			<meta charset="utf-8"/>
			<title>Error { status }</title>
			<script src="https://cdn.tailwindcss.com"></script>
		</head>
		<body class="bg-slate-50 text-slate-900">
			<main class="mx-auto max-w-md px-4 py-16">
				<ErrorBlock status={status} msg={msg}/>
			</main>
		</body>
	</html>
}

component ErrorBlock(status int, msg string) {
	<div class="rounded-lg border border-red-200 bg-red-50 p-6 text-center">
		<div class="text-4xl font-semibold text-red-700">{ status }</div>
		<p class="mt-2 text-red-700">{ msg }</p>
		<a class="mt-4 inline-block text-sm text-red-700 underline" href="/">
			Back to home
		</a>
	</div>
}
