package plain

component Home() {
	<button hx-on:click="go()" data-hint="hx-get">not a request</button>
}
