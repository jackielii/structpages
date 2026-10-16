package app

import "github.com/jackielii/structpages"

// Requests covers elements making requests of their own, and value forms.
component (p ace) Requests() {
	<div id={detail.Content |> id} hx-get={detail{} |> url} hx-trigger="load">lazy load from detail</div>
	<form hx-post={detail{} |> url} hx-target={detail.Content |> target}>post to detail</form>
	<div id={detail.Content |> id} hx-get={ace{} |> url}>reload from ace</div>
	<div hx-get={structpages.URLFor(ctx, ace{})} hx-target={detail.Content |> target}>hand-written URLFor</div>
	<div hx-get="/ace" hx-target={detail.Content |> target}>literal URL</div>
	<div hx-get={ace{} |> url} hx-post={detail{} |> url} hx-target={detail.Content |> target}>one elsewhere</div>
	<div
		class="multi"
		hx-target={detail.Content |> target}
	>multi-line staged</div>
	<div id={structpages.ID(ctx,
		detail.Content)}>multi-line hand-written</div>
	<div id={structpages.ID(ctx, detail.Content) /* why */}>comment</div>
	<div id={wrap.Content |> id}>promoted method expression: wrap's</div>
}

component (p wrap) Page() {
	<div id={wrap.Content |> id}>own promoted method expression</div>
	<div id={p.Content |> id}>promoted method value: detail's</div>
}
