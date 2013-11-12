package es

type Request struct {
	Query  *Query `json:"query,omitempty"`
	Size   int    `json:"size,omitempty"`
	Facets `json:"facets,omitempty"`
	*Sort  `json:"sort,omitempty"`
}

func (request *Request) AddFacet(key string, facet *Facet) {
	if request.Facets == nil {
		request.Facets = Facets{}
	}
	request.Facets[key] = facet
}

type DateHistogram struct {
	Field    string `json:"field,omitempty"`
	Interval string `json:"interval,omitempty"`
}

type RequestFacet struct {
	*Terms         `json:"terms,omitempty"`
	*DateHistogram `json:"date_histogram,omitempty"`
}

type Sort map[string]*SortCriteria

type SortCriteria struct {
	Order string `json:"order,omitempty"`
}

const (
	Asc  = "asc"
	Desc = "desc"
)
