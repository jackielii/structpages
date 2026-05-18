package main

import (
	"errors"
	"fmt"

	"github.com/jackielii/structpages"
)

// validateURLs exercises every URL this app can generate, returning
// any that fail to resolve as a single joined error. Call it once at
// startup (see main.go); a dangling URL kills the boot instead of the
// first request that tries to use it.
//
// This is the production-safety twin of integration_test.go. The test
// validates in CI; the validator validates in `main` so a broken
// deploy refuses to start serving. Both are cheap; running both is
// belt-and-suspenders for "no dangling URLs in production."
//
// What to put in here:
//
//  1. One call per URL family the app generates — covers every []any
//     chain and any bare typed lookup. Renames of fields or parent
//     types fail loud here.
//  2. One call per Ref the app uses — covers cross-package routes
//     where the page type can't be imported (see urlForByRef in
//     urls.go for the rationale).
//  3. Routes that take params: pass a representative value via the
//     map[string]any. The validator doesn't care if the value exists
//     in your data; it only checks the URL template resolves.
func validateURLs(sp *structpages.StructPages) error {
	var errs []error
	check := func(label string, gen func() (string, error)) {
		if _, err := gen(); err != nil {
			errs = append(errs, fmt.Errorf("%s: %w", label, err))
		}
	}

	// 1. URLs generated via the typed chain form. We call the
	//    underlying URLFor primitive here, not the helpers in urls.go
	//    — the integration test exercises helpers end-to-end; this
	//    exercises the primitive directly, so a typo in either layer
	//    surfaces.
	check("home", func() (string, error) {
		return sp.URLFor(homePage{})
	})
	for _, group := range []struct {
		name   string
		parent any
	}{
		{"foundations", foundationsRoot{}},
		{"components", componentsRoot{}},
		{"patterns", patternsRoot{}},
	} {
		check(group.name+" index", func() (string, error) {
			return sp.URLFor([]any{group.parent, groupIndex{}})
		})
		check(group.name+" detail", func() (string, error) {
			return sp.URLFor([]any{group.parent, entryPage{}},
				map[string]any{"slug": "sample"})
		})
		check(group.name+" index with tab", func() (string, error) {
			return sp.URLFor([]any{group.parent, groupIndex{}, "?tab={tab}"},
				map[string]any{"tab": "overview"})
		})
	}

	// 2. Refs. None *required* in this single-package demo, but the
	//    pattern is the same — and we exercise one to demonstrate.
	//    In a real app, every cross-package Ref gets a check here.
	check("components index via Ref qualified", func() (string, error) {
		return sp.URLFor(structpages.Ref("Components.Index"))
	})
	check("foundations group via Ref single name", func() (string, error) {
		return sp.URLFor(structpages.Ref("Foundations"))
	})

	return errors.Join(errs...)
}
