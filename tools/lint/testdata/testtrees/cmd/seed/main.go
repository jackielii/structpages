// Command seed is an HTTP client of the app. It mounts nothing, so the
// app's routes it calls are a public contract, not drift.
package main

import "net/http"

func main() {
	_, _ = http.Get("http://localhost:8080" + "/orbit/data")
}
