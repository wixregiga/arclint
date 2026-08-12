package app

import (
	"example.com/library/internal/borrowbook"
	borrowhttp "example.com/library/internal/borrowbook/http"
	"example.com/library/internal/member"
	"example.com/library/internal/returnbook"
	returnhttp "example.com/library/internal/returnbook/http"
	"example.com/library/internal/shared"
)

// Run wires config -> shared adapters -> use cases -> HTTP. No business
// rules live here.
func Run() {
	adapters := shared.NewAdapters()
	var members member.Repo = adapters.Members
	borrow := borrowbook.New(members)
	giveBack := returnbook.New(members)
	borrowhttp.Mount(borrow)
	returnhttp.Mount(giveBack)
}
