package components

import (
	"context"
	"fmt"

	"github.com/a-h/templ"
	"github.com/jackielii/structpages"
)

// URL wraps structpages.URLFor to return a templ.SafeURL so it can be passed
// to href/action attributes without templ's URL-context sanitization warning.
func URL(ctx context.Context, page any, args ...any) (templ.SafeURL, error) {
	s, err := structpages.URLFor(ctx, page, args...)
	return templ.SafeURL(s), err
}

// PaginationProps describes a paginated control. URL builds the link for a
// given page number; callers typically wrap structpages.URLFor with the
// "?page={page}" template form.
type PaginationProps struct {
	Page     int
	PageSize int
	Total    int
	URL      func(page int) (string, error)
}

func (p PaginationProps) Pages() int {
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

func (p PaginationProps) Range() string {
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

func (p PaginationProps) HasPrev() bool { return p.Page > 1 }
func (p PaginationProps) HasNext() bool { return p.Page < p.Pages() }
