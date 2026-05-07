// Package blog hosts the public reader: home page, post detail, category
// archive, search, and the comment form action. All field names below become
// global structpages.Ref keys, so the layout package can link to them with
// Ref("home"), Ref("search"), etc., without importing this package.
package blog

// Pages is the routable surface of the blog feature. main mounts it at "/".
type Pages struct {
	home       homePage       `route:"/{$} Home"`
	post       postPage       `route:"/posts/{slug} Post"`
	category   categoryPage   `route:"/categories/{slug} Category"`
	search     searchPage     `route:"GET /search Search"`
	addComment commentHandler `route:"POST /posts/{slug}/comments Add Comment"`
}
