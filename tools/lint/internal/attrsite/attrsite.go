// Package attrsite describes the expression attributes of a template
// (.gsx or .templ) by the position, in the generated Go file, of the
// call that produces each attribute's value. The [region] check joins
// a structpages.ID / IDTarget call to the attribute it feeds through
// it. gsxscan and templscan each build the index from their own
// template AST and codegen's position mapping.
package attrsite

// Site is a 1-indexed line and column.
type Site struct {
	Line, Col int
}

// Attr is an element attribute whose value is a Go expression.
type Attr struct {
	// Name is the attribute name as authored ("hx-target:inherited").
	Name string
	// Tag is the tag of the element carrying the attribute.
	Tag string
	// Component reports that Tag is a component tag: its attributes
	// are props, not HTML attributes of an element.
	Component bool
	// Staged reports that the value is the result of a pipeline stage
	// (gsx `|> id`); the indexed call is then that last stage's.
	// Otherwise the value is the expression itself, and the indexed call
	// must be all of Expr.
	Staged bool
	// Expr is the attribute's expression source, as authored (the
	// pipeline seed when Staged).
	Expr string
	// Stages is the number of pipeline stages (gsx); 0 when Staged is
	// false.
	Stages int
	// Src is the expression's position in the template file.
	Src Site
	// Requests are the element's htmx request attributes (hx-get …
	// hx-delete, in any htmx spelling), including those in in-tag
	// conditionals.
	Requests []Request
	// Spread reports that the element also takes attributes from a
	// spread ({ attrs... }), whose names are only known at runtime.
	Spread bool
}

// Request is an htmx request attribute of an element.
type Request struct {
	// Name is the attribute name as authored.
	Name string
	// Expr reports that the value is a Go expression; when false (a
	// literal value, a gsx f`…` template) no URLFor call produces it.
	Expr bool
	// Site, Staged, Stages and Source locate the call producing an
	// expression value, as for Attr.
	Site   Site
	Staged bool
	Stages int
	Source string
}

// Value is an element attribute value as the template writes it: its
// literal text and Go expressions, in order. A gsx f`…@{x}…` template
// has a part per text run and interpolation; an expression attribute
// (attr={x}) has one part.
type Value struct {
	// Name is the attribute name as authored.
	Name string
	// Tag is the tag of the element carrying the attribute.
	Tag string
	// Component reports that Tag is a component tag: its attributes
	// are props, not HTML attributes of an element.
	Component bool
	Parts     []Part
}

// Part is one piece of an attribute value: literal text or a Go
// expression.
type Part struct {
	// Text is the text of a literal part.
	Text string
	// Expr reports that the part is a Go expression. Site, Staged,
	// Stages and Source locate the call or expression producing it, as
	// for Attr; Src is where it starts in the template.
	Expr   bool
	Site   Site
	Staged bool
	Stages int
	Source string
	Src    Site
}
