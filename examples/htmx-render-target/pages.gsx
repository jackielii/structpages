package main

// Shared standalone function components (can be used across multiple pages).
// These demonstrate the power of RenderTarget — no wrapper methods needed.

component UserStatsWidget(stats UserStats) {
	<div class="widget">
		<h3>User Statistics</h3>
		<p>Active Users: { stats.ActiveUsers }</p>
		<p>New Today: { stats.NewToday }</p>
		<button
			type="button"
			hx-get={dashboard{} |> url}
			hx-target={UserStatsWidget |> target}
		>
			Refresh Stats
		</button>
	</div>
}

component SalesChartWidget(data SalesData) {
	<div class="widget">
		<h3>Sales Chart</h3>
		<div class="chart">
			{ for _, point := range data.Points {
				<div
					class="bar"
					data-h={point.Value}
					style="width: 30px; background: blue; display: inline-block; margin: 2px;"
				></div>
			} }
		</div>
		<p>Total Sales: ${ data.Total |> format("%.2f") }</p>
		<button
			type="button"
			hx-get={dashboard{} |> url}
			hx-target={SalesChartWidget |> target}
		>
			Refresh Sales
		</button>
	</div>
}

component NotificationsList(notifications []Notification) {
	<div class="widget">
		<h3>Recent Notifications</h3>
		<ul>
			{ for _, n := range notifications {
				<li>
					{ n.Message } <small>({ n.Time.Format("15:04") })</small>
				</li>
			} }
		</ul>
		<button
			type="button"
			hx-get={dashboard{} |> url}
			hx-target={NotificationsList |> target}
		>
			Refresh Notifications
		</button>
	</div>
}

// Dashboard page.

type dashboard struct{}

component (p dashboard) Page(props dashboardData) {
	<Html>
		<h1>Dashboard</h1>
		<p>
			This example demonstrates the RenderTarget API with standalone function components.
		</p>
		<p>
			Click "Refresh" buttons to see HTMX partial updates — each widget loads only its own data!
		</p>
		<div
			style="display: grid; grid-template-columns: repeat(auto-fit, minmax(300px, 1fr)); gap: 1rem; margin-top: 2rem;"
		>
			<div id={UserStatsWidget |> id}>
				<UserStatsWidget { props.Stats... }/>
			</div>
			<div id={SalesChartWidget |> id}>
				<SalesChartWidget { props.Sales... }/>
			</div>
			<div id={NotificationsList |> id}>
				<NotificationsList notifications={props.Notifications}/>
			</div>
		</div>
		<div
			style="margin-top: 2rem; padding: 1rem; background: #f0f0f0; border-radius: 4px;"
		>
			<h4>How it works:</h4>
			<ul>
				<li>
					✅ <strong>
						Standalone functions
					</strong> — UserStatsWidget, SalesChartWidget, NotificationsList are shared components
				</li>
				<li>
					✅ <strong>
						Conditional loading
					</strong> — Props checks target.Is() and loads only needed data
				</li>
				<li>
					✅ <strong>
						RenderComponent (direct)
					</strong> — construct the gsx component with its props struct and pass directly
				</li>
				<li>
					✅ <strong>
						No wrapper methods
					</strong> — No need to create dashboard.UserStats() method!
				</li>
				<li>
					✅ <strong>
						HTMX integration
					</strong> — HTMXRenderTarget automatically handles partial updates
				</li>
			</ul>
		</div>
	</Html>
}

// Html is the full-page layout.
component Html() {
	<!DOCTYPE html>
	<html lang="en">
		<head>
			<link rel="stylesheet" href="https://unpkg.com/missing.css@1.1.3"/>
			<script src="https://unpkg.com/htmx.org@2.0.4"></script>
			<title>RenderTarget API Example</title>
			<style>
				.widget {
					padding: 1rem;
					border: 1px solid #ddd;
					border-radius: 8px;
					background: white;
				}
				.widget h3 {
					margin-top: 0;
				}
				.chart {
					display: flex;
					align-items: flex-end;
					height: 150px;
					margin: 1rem 0;
				}
			</style>
		</head>
		<body>
			<main>{ children }</main>
			<script>
				// Optional: Add some basic HTMX event listeners for debugging
				document.body.addEventListener('htmx:beforeRequest', (evt) => {
					console.log('HTMX Request:', evt.detail);
				});
				document.body.addEventListener('htmx:afterRequest', (evt) => {
					console.log('HTMX Response:', evt.detail);
				});
			</script>
		</body>
	</html>
}

component ErrorPage(err error) {
	<Html>
		<ErrorComp err={err}/>
	</Html>
}

component ErrorComp(err error) {
	<h1>Error</h1>
	<p>{ err.Error() }</p>
}
