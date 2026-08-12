package app

import (
	"example.com/library/internal/borrowbook"
	borrowhttp "example.com/library/internal/borrowbook/http"
	"example.com/library/internal/member"
	"example.com/library/internal/shared"
)

// Run wires config -> shared adapters -> use cases -> HTTP. The returnbook
// feature exists but was never wired in: repo.features-wired must fire.
func Run() {
	adapters := shared.NewAdapters()
	var members member.Repo = adapters.Members
	borrow := borrowbook.New(members)
	borrowhttp.Mount(borrow)
}
