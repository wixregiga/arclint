package syntax

import "os"

// exhaustive reports a run that asks for every case of the long property
// sweeps, as make test-exhaustive does with ARCLINT_EXHAUSTIVE=1. Other
// runs, the repository gate and CI included, take a fixed, evenly spaced
// sample of each sweep, so every change still meets every kind of case.
func exhaustive() bool { return os.Getenv("ARCLINT_EXHAUSTIVE") == "1" }

// sampled keeps every case of a sweep in an exhaustive run, and every
// stride-th case, the first included, otherwise.
func sampled(i, stride int) bool { return exhaustive() || i%stride == 0 }
