package silon

import (
	"context"
	"iter"
	"net/url"
)

// Page is one page of a cursor-paginated list endpoint
// ({"results": [...], "next": url|null, "previous": url|null}).
//
// NextPage never follows the opaque "next" URL directly: only its query
// parameters are extracted and merged over the original parameters, and
// the original path is re-requested against the configured base URL, so a
// proxy-rewritten hostname cannot hijack pagination.
type Page[T any] struct {
	// Results holds this page's items.
	Results []T

	// Next is the opaque next-page URL advertised by the API (nil on the
	// last page). Only its query parameters are ever used.
	Next *string

	// Previous is the opaque previous-page URL, when present.
	Previous *string

	client *Client
	path   string
	params url.Values
}

type pageEnvelope[T any] struct {
	Results  []T     `json:"results"`
	Next     *string `json:"next"`
	Previous *string `json:"previous"`
}

// fetchPage GETs path with params and wraps the cursor envelope in a Page.
func fetchPage[T any](ctx context.Context, c *Client, path string, params url.Values) (*Page[T], error) {
	var env pageEnvelope[T]
	if err := c.get(ctx, path, params, &env); err != nil {
		return nil, err
	}
	return &Page[T]{
		Results:  env.Results,
		Next:     env.Next,
		Previous: env.Previous,
		client:   c,
		path:     path,
		params:   params,
	}, nil
}

// HasNextPage reports whether a following page exists.
func (p *Page[T]) HasNextPage() bool {
	return p.Next != nil && *p.Next != ""
}

// NextPage fetches the following page. Calling it on the last page
// (HasNextPage() == false) returns an *Error.
func (p *Page[T]) NextPage(ctx context.Context) (*Page[T], error) {
	if !p.HasNextPage() {
		return nil, &Error{Message: "This page has no next page; check HasNextPage first."}
	}
	nextURL, err := url.Parse(*p.Next)
	if err != nil {
		return nil, &Error{Message: "Could not parse next page URL: " + err.Error()}
	}
	merged := url.Values{}
	for k, vs := range p.params {
		merged[k] = append([]string(nil), vs...)
	}
	for k, vs := range nextURL.Query() {
		merged[k] = append([]string(nil), vs...)
	}
	return fetchPage[T](ctx, p.client, p.path, merged)
}

// All returns an iterator that lazily walks every item on this page and
// all following pages, fetching each next page only when needed:
//
//	for event, err := range page.All(ctx) {
//		if err != nil {
//			return err
//		}
//		...
//	}
//
// A page-fetch failure is yielded once as a non-nil error and ends the
// iteration.
func (p *Page[T]) All(ctx context.Context) iter.Seq2[T, error] {
	return func(yield func(T, error) bool) {
		page := p
		for {
			for _, item := range page.Results {
				if !yield(item, nil) {
					return
				}
			}
			if !page.HasNextPage() {
				return
			}
			next, err := page.NextPage(ctx)
			if err != nil {
				var zero T
				yield(zero, err)
				return
			}
			page = next
		}
	}
}
