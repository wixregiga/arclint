package http

import (
	nethttp "net/http"

	"example.com/library/internal/borrowbook"
)

// Mount registers the feature's driving adapter.
func Mount(u *borrowbook.UseCase) {
	nethttp.HandleFunc("/borrowbook", func(w nethttp.ResponseWriter, r *nethttp.Request) {})
	_ = u
}
