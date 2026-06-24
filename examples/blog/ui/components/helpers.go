package components

import (
	"context"
	"fmt"

	"github.com/jackielii/structpages"
)

// URL wraps structpages.URLFor. gsx auto-sanitizes URL-context attributes
// (href/action/src/hx-*), so a plain (string, error) is enough — and it
// auto-unwraps in attribute position, dropping the explicit error handling.
func URL(ctx context.Context, page any, args ...any) (string, error) {
	return structpages.URLFor(ctx, page, args...)
}

// PageNav describes a paginated control. URL builds the link for a
// given page number; callers typically wrap structpages.URLFor with the
// "?page={page}" template form.
type PageNav struct {
	Page     int
	PageSize int
	Total    int
	URL      func(page int) (string, error)
}

func (p PageNav) Pages() int {
	if p.PageSize <= 0 {
		return 1
	}
	pages := p.Total / p.PageSize
	if p.Total%p.PageSize != 0 {
		pages++
	}
	if pages < 1 {
		pages = 1
	}
	return pages
}

func (p PageNav) Range() string {
	if p.Total == 0 {
		return "No results"
	}
	start := (p.Page-1)*p.PageSize + 1
	end := start + p.PageSize - 1
	if end > p.Total {
		end = p.Total
	}
	return fmt.Sprintf("%d–%d of %d", start, end, p.Total)
}

func (p PageNav) HasPrev() bool { return p.Page > 1 }
func (p PageNav) HasNext() bool { return p.Page < p.Pages() }
