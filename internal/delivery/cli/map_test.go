package cli

import (
	"bytes"
	"errors"
	"io"
	"strings"
	"testing"

	"github.com/wixregiga/arclint/internal/application"
	"github.com/wixregiga/arclint/internal/domain/vocab"
)

// memoryKnowledge is the vocabulary repository port held in memory: a
// recorded language, or none.
type memoryKnowledge struct {
	recording vocab.DomainRecording
	language  vocab.UbiquitousLanguage
	found     bool
}

func (k memoryKnowledge) Recording() vocab.DomainRecording { return k.recording }
func (k memoryKnowledge) RecordedLanguage() (vocab.UbiquitousLanguage, bool, error) {
	return k.language, k.found, nil
}
func (memoryKnowledge) Record(vocab.UbiquitousLanguage) error { return nil }

// recordedOverview composes the overview use case over the language
// held in memory; a nil language is a recording with no file.
func recordedOverview(t *testing.T, language *vocab.UbiquitousLanguage) application.GetDomainOverview {
	t.Helper()
	recording, err := vocab.NewDomainRecording(vocab.DefaultDomainRecordingPath)
	if err != nil {
		t.Fatal(err)
	}
	knowledge := memoryKnowledge{recording: recording}
	if language != nil {
		knowledge.language, knowledge.found = *language, true
	}
	overview, err := application.NewGetDomainOverview(knowledge)
	if err != nil {
		t.Fatal(err)
	}
	return overview
}

func schedulingLanguage(t *testing.T) *vocab.UbiquitousLanguage {
	t.Helper()
	lang, err := vocab.NewUbiquitousLanguage("clinic", "Booking visits.", []vocab.BoundedContext{{
		Name:       "scheduling",
		Definition: "Where visits are booked.",
		Aggregates: []vocab.Aggregate{{Name: "AppointmentBook", Definition: "The book of visits.", Identity: "AppointmentBookId"}},
	}}, nil)
	if err != nil {
		t.Fatal(err)
	}
	return &lang
}

// fakeMapServer records what the command asked of its port and reports
// a URL as a bound server would.
type fakeMapServer struct {
	address string
	url     string
	err     error
}

func (f *fakeMapServer) Serve(address string, listening func(string) error) error {
	f.address = address
	if f.err != nil {
		return f.err
	}
	return listening(f.url)
}

func serveCommand(t *testing.T, command Command) Command {
	t.Helper()
	for _, sub := range command.Subcommands {
		if sub.Name == "serve" {
			return sub
		}
	}
	t.Fatal("map has no serve subcommand")
	return Command{}
}

func TestMapServeReportsTheURLThroughTheRenderer(t *testing.T) {
	server := &fakeMapServer{url: "http://127.0.0.1:7380"}
	renderer := &recordingRenderer{}
	serve := serveCommand(t, NewMapCommand(recordedOverview(t, schedulingLanguage(t)), server, renderer))
	if err := serve.Run(Context{Flags: map[string]string{flagListen: "127.0.0.1:0"}, Stdout: io.Discard}); err != nil {
		t.Fatal(err)
	}
	if server.address != "127.0.0.1:0" {
		t.Fatalf("address = %q", server.address)
	}
	if len(renderer.reports) != 1 {
		t.Fatalf("reports = %d, want 1", len(renderer.reports))
	}
	report, ok := renderer.reports[0].(MapServeReport)
	if !ok {
		t.Fatalf("report type = %T, want MapServeReport", renderer.reports[0])
	}
	want := MapServeReport{URL: "http://127.0.0.1:7380", Project: "clinic", Source: vocab.DefaultDomainRecordingPath}
	if report != want {
		t.Fatalf("report = %+v, want %+v", report, want)
	}
}

func TestMapServeListensOnLoopbackByDefault(t *testing.T) {
	serve := serveCommand(t, NewMapCommand(application.GetDomainOverview{}, &fakeMapServer{}, &recordingRenderer{}))
	if len(serve.Flags) != 1 || serve.Flags[0].Name != flagListen {
		t.Fatalf("flags = %+v", serve.Flags)
	}
	if serve.Flags[0].Default != "127.0.0.1:7380" {
		t.Fatalf("default address = %q, want the loopback interface", serve.Flags[0].Default)
	}
}

func TestMapServeWithoutARecordedLanguageReportsItsSource(t *testing.T) {
	server := &fakeMapServer{url: "http://127.0.0.1:7380"}
	renderer := &recordingRenderer{}
	serve := serveCommand(t, NewMapCommand(recordedOverview(t, nil), server, renderer))
	if err := serve.Run(Context{Flags: map[string]string{flagListen: defaultListen}, Stdout: io.Discard}); err != nil {
		t.Fatal(err)
	}
	if server.address != defaultListen || len(renderer.reports) != 1 {
		t.Fatalf("server = %+v, reports = %v", server, renderer.reports)
	}
	report, ok := renderer.reports[0].(MapServeReport)
	if !ok || report.Source != vocab.DefaultDomainRecordingPath || report.URL != server.url || report.Project != "" {
		t.Fatalf("report = %+v", renderer.reports[0])
	}
}

type brokenKnowledge struct{ memoryKnowledge }

func (brokenKnowledge) RecordedLanguage() (vocab.UbiquitousLanguage, bool, error) {
	return vocab.UbiquitousLanguage{}, true, errors.New("malformed recording")
}

func TestMapServeRefusesAMalformedRecording(t *testing.T) {
	overview, err := application.NewGetDomainOverview(brokenKnowledge{})
	if err != nil {
		t.Fatal(err)
	}
	server := &fakeMapServer{}
	serve := serveCommand(t, NewMapCommand(overview, server, &recordingRenderer{}))
	err = serve.Run(Context{Flags: map[string]string{flagListen: defaultListen}, Stdout: io.Discard})
	if err == nil || !strings.Contains(err.Error(), "malformed recording") || server.address != "" {
		t.Fatalf("err = %v, server = %+v", err, server)
	}
}

func TestMapServeWrapsAServerFailureAsConfiguration(t *testing.T) {
	server := &fakeMapServer{err: errors.New("listen on 127.0.0.1:7380: address already in use")}
	serve := serveCommand(t, NewMapCommand(recordedOverview(t, schedulingLanguage(t)), server, &recordingRenderer{}))
	err := serve.Run(Context{Flags: map[string]string{}, Stdout: io.Discard})
	var exit *ExitError
	if !errors.As(err, &exit) || exit.Code != ExitConfigError || !strings.Contains(exit.Message, "address already in use") {
		t.Fatalf("err = %v", err)
	}
}

func TestMapServeFailsWhenTheReportCannotBeWritten(t *testing.T) {
	writeErr := errors.New("write failed")
	server := &fakeMapServer{url: "http://127.0.0.1:7380"}
	serve := serveCommand(t, NewMapCommand(recordedOverview(t, schedulingLanguage(t)), server, &recordingRenderer{err: writeErr}))
	var out bytes.Buffer
	if err := serve.Run(Context{Flags: map[string]string{}, Stdout: &out}); !errors.Is(err, writeErr) {
		t.Fatalf("err = %v, want the renderer's error", err)
	}
}
