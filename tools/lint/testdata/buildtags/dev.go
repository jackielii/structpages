package main

import (
	"context"

	"github.com/jackielii/structpages"
)

// devPage's type is always compiled; whether it is MOUNTED depends on the build
// tag (dev_mounted.go vs dev_stub.go).
type devPage struct{}

// linkDev is always compiled and URLFors the dev page. With -tags=devtools the
// page is mounted and this resolves; without it the lint must flag it.
func linkDev(ctx context.Context) (string, error) {
	return structpages.URLFor(ctx, devPage{})
}
