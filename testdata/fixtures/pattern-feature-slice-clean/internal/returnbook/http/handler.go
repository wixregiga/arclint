package http

import (
	nethttp "net/http"

	"example.com/library/internal/returnbook"
)

// Mount registers the feature's driving adapter.
func Mount(u *returnbook.UseCase) {
	nethttp.HandleFunc("/returnbook", func(w nethttp.ResponseWriter, r *nethttp.Request) {})
	_ = u
}
