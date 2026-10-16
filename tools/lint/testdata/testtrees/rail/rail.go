// Package rail holds nav refs as data: they reach URLFor only at
// runtime, so the linter checks them against every tree the package's
// binaries can mount.
package rail

import "github.com/jackielii/structpages"

type Item struct {
	Ref structpages.Ref
}
