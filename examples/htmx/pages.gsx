package main

import (
  "fmt"

	"github.com/jackielii/structpages"
)

type index struct {
	product `route:"/product Product"`
	team    `route:"/team Team"`
	contact `route:"/contact Contact"`
  throw   `route:"/throw Throw"`
}
type product struct{}
type team struct{}
type contact struct{}
type throw struct{}

component (p index) Page()   { <Layout><p.Main/></Layout> }
component (p index) Main() {
	<h1>Welcome to the Index Page</h1>
	<p>Navigate to the product, team, or contact pages using the links below:</p>
  <a hx-get={ urlFor(ctx, throw{}) } hx-target="#main">Throw (Err)</a>
}

component (p product) Page() { <Layout><p.Main/></Layout> }
component (p product) Main() {
	<h1>Product Page</h1>
	<p>This is the product page.</p>
}

component (p team) Page() { <Layout><p.Main/></Layout> }
component (p team) Main() {
	<h1>Team Page</h1>
	<p>This is the team page.</p>
}

component (p contact) Page() { <Layout><p.Main/></Layout> }
component (p contact) Main() {
	<h1>Contact Page</h1>
	<p>This is the contact page.</p>
}

func errFunc() (string, error ) { return "", fmt.Errorf("this is an error") }
component (p throw) Page() { <p>{errFunc()}</p> }


component Layout() {
	<!DOCTYPE html>
	<html lang="en">
		<head>
			<link rel="stylesheet" href="https://unpkg.com/missing.css@1.1.3" />
			<script src="https://unpkg.com/htmx.org@2.0.4"></script>
			<title>HTMX Example</title>
		</head>
		<body>
			<header class="navbar">
				<nav>
					<ul role="list" hx-push-url="true" hx-target="#main">
						<li><a hx-get={ urlFor(ctx, index{}) }>Home</a></li>
						<li><a hx-get={ urlFor(ctx, product{}) }>Product</a></li>
						<li><a hx-get={ urlFor(ctx, team{}) }>Team</a></li>
						<li><a hx-get={ urlFor(ctx, contact{}) }>Contact</a></li>
					</ul>
				</nav>
			</header>
			<main id="main">{children}</main>
		</body>
	</html>
}

component ErrorPage(err error) { <Layout><ErrorComp err={err} /></Layout> }

component ErrorComp(err error) {
	<h1>Error</h1>
	<p>{ err.Error() }</p>
}

var urlFor = structpages.URLFor
