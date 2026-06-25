package main

import "github.com/jackielii/structpages"

// Page structs + route tags are plain Go — pass through unchanged.
type index struct {
	product `route:"/product Product"`
	team    `route:"/team Team"`
	contact `route:"/contact Contact"`
}
type product struct{}
type team struct{}
type contact struct{}

component (p index) Page() {
	<Layout>
		<h1>Welcome to the Index Page</h1>
		<p>Navigate to the product, team, or contact pages using the links below:</p>
	</Layout>
}

component (p product) Page() {
	<Layout>
		<h1>Product Page</h1>
		<p>This is the product page.</p>
	</Layout>
}

component (p team) Page() {
	<Layout>
		<h1>Team Page</h1>
		<p>This is the team page.</p>
	</Layout>
}

component (p contact) Page() {
	<Layout>
		<h1>Contact Page</h1>
		<p>This is the contact page.</p>
	</Layout>
}

component Layout() {
	<!DOCTYPE html>
	<html lang="en">
		<head>
			<link rel="stylesheet" href="https://unpkg.com/missing.css@1.1.3" />
			<title>Simple Example</title>
		</head>
		<body>
			<header class="navbar">
				<nav>
					<ul role="list">
						<li><a href={ urlFor(ctx, index{}) }>Home</a></li>
						<li><a href={ urlFor(ctx, product{}) }>Product</a></li>
						<li><a href={ urlFor(ctx, team{}) }>Team</a></li>
						<li><a href={ urlFor(ctx, contact{}) }>Contact</a></li>
					</ul>
				</nav>
			</header>
			<main>{children}</main>
		</body>
	</html>
}

var urlFor = structpages.URLFor
