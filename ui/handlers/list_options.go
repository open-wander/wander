package handlers

import "net/url"

type listViewOptions struct {
	search  string
	sort    string
	dir     string
	page    int
	perPage int
}

func parseListViewOptions(query url.Values, defaultSort, defaultDir string) listViewOptions {
	sortColumn := query.Get("sort")
	if sortColumn == "" {
		sortColumn = defaultSort
	}
	sortDir := query.Get("dir")
	if sortDir == "" {
		sortDir = defaultDir
	}
	return listViewOptions{
		search:  query.Get("search"),
		sort:    sortColumn,
		dir:     sortDir,
		page:    positiveBoundedInt(query.Get("page"), 1, int(^uint(0)>>1)),
		perPage: positiveBoundedInt(query.Get("per_page"), 25, 100),
	}
}
