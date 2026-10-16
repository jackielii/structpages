package fixture

component Partial() {
	<div hx-on:click="x()" data-note="hx-get">not htmx requests</div>
	<hx-partial id="main">swap</hx-partial>
}
