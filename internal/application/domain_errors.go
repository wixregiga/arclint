package application

import "errors"

// ErrDomainUsage marks invalid domain command usage (unsupported type,
// missing name, mutually exclusive flags). Delivery maps it to CLI
// exit 2.
var ErrDomainUsage = errors.New("invalid domain command usage")
