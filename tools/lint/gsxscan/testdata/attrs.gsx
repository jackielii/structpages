package fixture

import "fmt"

component Attrs(id int, name string) {
	<a href="/login">static</a>
	<a href={"/" + "admin"}>concat of literals</a>
	<a href={fmt.Sprintf("/users/%s", name)}>sprintf</a>
	<a hx-get={"/api/items"}>expr literal</a>
	<a hx-post=f`/items/@{id}`>interpolation</a>
	<form action="/submit"></form>
	<a href="https://example.com/x">external</a>
	<a href="//cdn.example.com/x">protocol-relative</a>
	<a title="/not-a-url-attr">other attr</a>
	<a href="#top">fragment</a>
	{ if id > 0 {
		<a href="/in-if">nested in control flow</a>
	} }
}
