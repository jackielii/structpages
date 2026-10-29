package app

import (
	"strconv"

	"github.com/jackielii/structpages"
)

component (p list) Page(rows []row, sp *structpages.StructPages, role string) {
	{{ baseID, err := structpages.ID(ctx, list.Row) }}
	{{
		if err != nil {
			return err
		}
	}}
	{{ baseTarget, err := structpages.IDTarget(ctx, list.Row) }}
	{{
		if err != nil {
			return err
		}
	}}
	{{ changed, err := structpages.ID(ctx, list.Row) }}
	{{
		if err != nil {
			return err
		}
	}}
	{{ changed = role }}
	<ul>
		{ for _, r := range rows {
			{{ instanceID := baseID + "-" + r.ID }}
			<li id=f`@{list.Row |> id}-@{r.ID}`>key appended</li>
			<li hx-target=f`@{list.Row |> target}-@{r.ID}`>target</li>
			<li hx-target:inherited=f`@{list.Row |> target}-@{r.ID}`>htmx 4</li>
			<li data-hx-target=f`@{list.Row |> target}-@{r.ID}`>data- prefix</li>
			<li id=f`@{list.Row |> id}-@{r.ID}-title`>key, then a role</li>
			<li id=f`@{list.Row |> id}-picker-@{r.ID}`>a role, then a key</li>
			<li id=f`@{list.Row |> id}@{r.ID}`>no separator</li>
			<li id=f`@{list.Row |> id}-@{strconv.Itoa(r.N)}`>a call</li>
			<li id=f`@{structpages.ID(ctx, list.Row)}-@{r.ID}`>hand-written ID</li>
			<li id=f`@{sp.ID(list.Row)}-@{r.ID}`>StructPages method</li>
			<li id={baseID + "-" + r.ID}>Go concatenation</li>
			<li id=f`@{baseID}-@{r.ID}`>variable from ID</li>
			<li id={instanceID}>variable holding the concatenation</li>
			<li hx-target={baseTarget + "-" + r.ID}>variable from IDTarget</li>
			<li hx-target=f`#@{list.Row |> id}-@{r.ID}`>"#" and ID</li>
			<li hx-target=f`closest @{list.Row |> target}-@{r.ID}`>extended selector</li>
			<li id=f`@{list.Row |> key(r.ID)}-@{r.N}`>keyed id extended</li>
			<li hx-target=f`@{list.Row |> keytarget(r.ID)}-@{r.N}`>keyed target extended</li>
			<li
				{ if r.N > 0 {
					id=f`@{list.Row |> id}-@{r.ID}`
				} }
			>
				in-tag if
			</li>
			<li
				id=f`@{list.Row |> id}-@{r.ID}`
			>
				multi-line
			</li>
			<li id=f`@{list.Row |> id}-document`>fixed role</li>
			<li id=f`@{list.Row |> id}-@{roleTitle}`>constant role</li>
			<li id={baseID + "-" + roleTitle}>constant role, Go</li>
			<li id=f`@{list.Row |> key(r.ID)}-title`>role on a keyed id</li>
			<li id={list.Row |> key(r.ID)}>Key</li>
			<li hx-target={list.Row |> keytarget(r.ID)}>KeyTarget</li>
			<li id={list.Row |> id}>ID</li>
			<li id={baseID}>variable from ID, alone</li>
			<li hx-target=f`@{list.Row |> target}-document`>fixed role target</li>
			<li hx-target=f`@{list.Row |> target} [data-id="@{r.ID}"]`>another compound</li>
			<li hx-target=f`@{list.Row |> target}-doc [data-id="@{r.ID}"]`>role, then another compound</li>
			<li hx-target=f`@{list.Row |> target}@{r.Sel}`>may start another selector</li>
			<li data-row=f`@{list.Row |> id}-@{r.ID}`>other attribute</li>
			<li aria-labelledby=f`@{list.Row |> id}-@{r.ID}-title`>other attribute</li>
			<li id=f`row-@{list.Row |> id}-@{r.ID}`>text before the generated id</li>
			<li id=f`@{list.Row |> target}-@{r.ID}`>IDTarget as id</li>
			<li hx-target=f`@{list.Row |> id}-@{r.ID}`>ID without "#"</li>
			<li id={changed + "-" + r.ID}>reassigned variable</li>
			<li id={role + "-" + r.ID}>no generated id</li>
			<Box id=f`@{list.Row |> id}-@{r.ID}`/>
			// structpages:lint:ignore keyed
			<li id=f`@{list.Row |> id}-@{r.ID}`>suppressed</li>
		} }
	</ul>
}

component (p list) Row() {
	<p>row</p>
}

component Box(id string) {
	<div id={id}></div>
}
