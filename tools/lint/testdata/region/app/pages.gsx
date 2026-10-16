package app

import (
	"ex/ui"

	"github.com/jackielii/structpages"
)

component (p ace) Page() {
	<main>
		<section id={[]any{Root{}, detail.Content} |> id}>chain</section>
		<div hx-target={detail.Content |> target}>method expression</div>
		<div hx-target:inherited={p.d.Content |> target}>bound value, htmx 4</div>
		<div data-hx-target={detail.Content |> target}>data- prefix</div>
		<div id={structpages.ID(ctx, detail.Content)}>hand-written</div>
		<hx-partial hx-target={detail.Content |> target}>out-of-band</hx-partial>
		<div data-region={detail.Content |> target}>other attribute</div>
		<div id={detail.Content |> target}>IDTarget as id</div>
		<div hx-target={detail.Content |> id}>ID as hx-target</div>
		<div id={ui.Toast |> id}>standalone</div>
		<div id={ace.Content |> id}>own component</div>
		<ui.Box id={detail.Content |> id}/>
		// structpages:lint:ignore region
		<div id={detail.Content |> id}>suppressed</div>
		{ if p.d == (detail{}) {
			<div id={detail.Content |> id}>in control flow</div>
		} }
	</main>
}

component (p ace) Content() {
	<p>ace</p>
}

component (p detail) Page() {
	<div id={detail.Content |> id}>
		<p.Content/>
	</div>
}

component (p detail) Content() {
	<p>detail</p>
}

component (p handled) Page() {
	<div id={detail.Content |> id}>handled in Props</div>
}

component (w widget) Body() {
	<div id={detail.Content |> id}>shared component</div>
}
