package fixture

component Suppressed() {
	// structpages:lint:ignore url-attr
	<a href="/login">bare Go comment above</a>
	{/* structpages:lint:ignore */}
	<a href="/admin">braced comment, all categories</a>
	// structpages:lint:ignore url-attr,ref
	<a href="/help">multi-category</a>
	<!-- structpages:lint:ignore url-attr -->
	<a href="/html-comment">HTML comment</a>
	<a
		// structpages:lint:ignore url-attr
		href="/in-attr-list"
	>attribute-list comment</a>
	// structpages:lint:ignore ref
	<a href="/other-category">different category does not suppress</a>
	<a href="/dashboard">not suppressed</a>
}
