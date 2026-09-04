package plain

import (
	"io"

	"github.com/wixregiga/arclint/internal/delivery/cli"
	"github.com/wixregiga/arclint/internal/delivery/cli/adapters/report/internal/out"
)

func writeInit(w io.Writer, r cli.InitReport) error {
	p := &out.Printer{W: w}
	p.Printf("wrote %s\nnext: declare your modules, then run `arclint check .`\n", r.Path)
	return p.Err
}

func writeArtifactStatus(w io.Writer, r cli.ArtifactStatusReport) error {
	p := &out.Printer{W: w}
	for _, a := range r.Writes {
		if a.Changed {
			p.Printf("wrote %s\n", a.Path)
			continue
		}
		p.Printf("%s already current\n", a.Path)
	}
	return p.Err
}

func writeBaselineCapture(w io.Writer, r cli.BaselineCaptureReport) error {
	p := &out.Printer{W: w}
	p.Printf("baseline captured: %d finding(s) across %d applied rule(s)\n",
		r.Result.Findings, r.Result.Rules)
	return p.Err
}

func writeBaselineRefresh(w io.Writer, r cli.BaselineRefreshReport) error {
	p := &out.Printer{W: w}
	p.Printf("baseline refreshed: %d finding(s) across %d applied rule(s), %d stale entr(ies) dropped\n",
		r.Result.Findings, r.Result.Rules, r.Result.RemovedStale)
	return p.Err
}

func writeSDKInit(w io.Writer, r cli.SDKInitReport) error {
	p := &out.Printer{W: w}
	for _, path := range r.Paths {
		p.Printf("wrote %s\n", path)
	}
	return p.Err
}
