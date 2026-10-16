package fixture

component Data(u string) {
	<a href="#top">top</a>
	{ if u != "" {
		<button DATA-HX-POST={u}>go</button>
	} }
}
