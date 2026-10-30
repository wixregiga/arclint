package main

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"sync"
	"testing"
	"time"

	"github.com/wixregiga/arclint/internal/application"
)

func startDraftServer(t *testing.T, root string) string {
	t.Helper()
	ctx, cancel := context.WithTimeout(context.Background(), time.Minute)
	t.Cleanup(cancel)
	cmd := exec.CommandContext(ctx, binPath, "map", "serve", "--addr", "127.0.0.1:0", "--format", "json")
	cmd.Dir = root
	out, err := cmd.StdoutPipe()
	if err != nil {
		t.Fatal(err)
	}
	var stderr bytes.Buffer
	cmd.Stderr = &stderr
	if err := cmd.Start(); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = cmd.Process.Kill(); _ = cmd.Wait() })
	var report struct {
		URL string `json:"url"`
	}
	if err := json.NewDecoder(out).Decode(&report); err != nil {
		t.Fatalf("map serve: %v", err)
	}
	if report.URL == "" {
		t.Fatal("map serve returned no URL")
	}
	return report.URL
}

func draftHTTP(base, method, endpoint string, body []byte, revision string) (int, []byte, error) {
	req, err := http.NewRequestWithContext(context.Background(), method, base+endpoint, bytes.NewReader(body))
	if err != nil {
		return 0, nil, fmt.Errorf("draft request: %w", err)
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("If-Match", revision)
	client := http.Client{Timeout: 5 * time.Second}
	res, err := client.Do(req)
	if err != nil {
		return 0, nil, fmt.Errorf("draft request: %w", err)
	}
	defer func() { _ = res.Body.Close() }()
	content, err := io.ReadAll(res.Body)
	if err != nil {
		return 0, nil, fmt.Errorf("draft response: %w", err)
	}
	return res.StatusCode, content, nil
}

func TestFeatureDraftsAcrossStudioProcesses(t *testing.T) {
	for _, example := range []struct{ directory, recording string }{
		{"docs/examples/go", "domain.arclint.yaml"},
		{"testing/boxoffice", "domain.arclint.cml"},
	} {
		t.Run(example.recording, func(t *testing.T) {
			root := t.TempDir()
			var original []byte
			for _, name := range []string{"rules.arclint.yaml", example.recording} {
				content, err := os.ReadFile(filepath.Join(repoCopy, example.directory, name))
				if err != nil {
					t.Fatal(err)
				}
				write(t, root, name, string(content))
				if name == example.recording {
					original = content
				}
			}
			first := startDraftServer(t, root)
			second := startDraftServer(t, root)
			request := func(base, method, endpoint string, body []byte, revision string, want int) []byte {
				t.Helper()
				status, content, err := draftHTTP(base, method, endpoint, body, revision)
				if err != nil || status != want {
					t.Fatalf("%s %s: %d %s (%v)", method, endpoint, status, content, err)
				}
				return content
			}
			overview := request(first, "GET", "/api/overview", nil, "", 200)
			var recorded struct {
				Counts struct {
					Contexts int `json:"contexts"`
				} `json:"counts"`
			}
			if err := json.Unmarshal(overview, &recorded); err != nil || recorded.Counts.Contexts == 0 {
				t.Fatalf("real recording not loaded: %s (%v)", overview, err)
			}
			content := request(first, "POST", "/api/feature", []byte(`{"title":"A shared feature","body":"世界 😀","references":[]}`), "", 200)
			var doc struct {
				Feature application.FeatureDraft `json:"feature"`
			}
			if err := json.Unmarshal(content, &doc); err != nil {
				t.Fatal(err)
			}
			old := doc.Feature
			if old.Body != "世界 😀" || old.ID == "" || old.Revision == "" {
				t.Fatalf("saved document: %+v", old)
			}
			reopened := request(second, "GET", "/api/feature?id="+old.ID, nil, "", 200)
			if !bytes.Equal(content, reopened) {
				t.Fatalf("another process reopened different content: %s", reopened)
			}
			request(first, "POST", "/api/feature", []byte(`{"title":"bad","body":"unchanged","references":[]} {}`), "", 400)
			request(first, "GET", "/api/feature?id=../domain.arclint.yaml", nil, "", 400)
			req, err := http.NewRequestWithContext(context.Background(), "POST", first+"/api/feature", bytes.NewReader([]byte(`{"title":"foreign","body":"","references":[]}`)))
			if err != nil {
				t.Fatal(err)
			}
			req.Header.Set("Content-Type", "application/json")
			req.Header.Set("Origin", "https://another-page.example")
			response, err := (&http.Client{Timeout: 5 * time.Second}).Do(req)
			if err != nil {
				t.Fatal(err)
			}
			_ = response.Body.Close()
			if response.StatusCode != 403 {
				t.Fatalf("foreign origin: %d", response.StatusCode)
			}
			start := make(chan struct{})
			outcomes := make(chan int, 2)
			failures := make(chan error, 2)
			var writers sync.WaitGroup
			for i, base := range []string{first, second} {
				next := old
				next.Body += fmt.Sprintf(" writer %d", i)
				payload, err := json.Marshal(next)
				if err != nil {
					t.Fatal(err)
				}
				writers.Go(func() {
					<-start
					status, _, err := draftHTTP(base, "POST", "/api/feature", payload, "")
					outcomes <- status
					failures <- err
				})
			}
			close(start)
			writers.Wait()
			close(outcomes)
			close(failures)
			for err := range failures {
				if err != nil {
					t.Fatal(err)
				}
			}
			counts := map[int]int{}
			for status := range outcomes {
				counts[status]++
			}
			if counts[200] != 1 || counts[409] != 1 {
				t.Fatalf("concurrent processes: %v", counts)
			}
			content = request(second, "GET", "/api/feature?id="+old.ID, nil, "", 200)
			if err := json.Unmarshal(content, &doc); err != nil {
				t.Fatal(err)
			}
			request(first, "DELETE", "/api/feature?id="+old.ID, nil, old.Revision, 409)
			request(second, "DELETE", "/api/feature?id="+old.ID, nil, doc.Feature.Revision, 200)
			list := request(first, "GET", "/api/features", nil, "", 200)
			if string(list) != `{"features":[]}` {
				t.Fatalf("remaining drafts: %s", list)
			}
			after, err := os.ReadFile(filepath.Join(root, example.recording))
			if err != nil || !bytes.Equal(original, after) {
				t.Fatalf("draft operations changed the recording: %v", err)
			}
		})
	}
}
