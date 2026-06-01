package queue

import (
	"context"
	"os"
	"testing"
)

// TestLiveGitHubQueueDrainSmoke is an OPT-IN live smoke (TIER 2) against a
// throwaway GitHub repo. It runs ONLY when QUEUE_DRAIN_SMOKE_REPO is set (e.g.
// Digital-Collective-Games/QueueDrainTestbed), so the normal `go test ./...` never
// touches the network or gh. It drives the REAL gh-backed provider + the real
// Consumer.DrainOnce against the live repo with a recording dispatcher, proving a
// live Queue=Ready issue is dispatched (no manual call) and a live Queue=Never
// issue is ignored. It performs NO GitHub writes (read-only); the Queue field is
// flipped out of band as a setup step.
func TestLiveGitHubQueueDrainSmoke(t *testing.T) {
	repo := os.Getenv("QUEUE_DRAIN_SMOKE_REPO")
	if repo == "" {
		t.Skip("set QUEUE_DRAIN_SMOKE_REPO to run the live GitHub queue-drain smoke")
	}
	provider, err := NewGitHubQueueProvider(repo, 0)
	if err != nil {
		t.Fatalf("build live provider: %v", err)
	}
	dispatcher := newFakeDispatcher()
	consumer := NewConsumer(repo, provider, dispatcher, fixedIdleSizer(4))

	result, err := consumer.DrainOnce(context.Background())
	if err != nil {
		t.Fatalf("live DrainOnce: %v", err)
	}
	t.Logf("live drain result: dispatched=%v parked=%v reclaimed=%v skipped=%v",
		result.Dispatched, result.Parked, result.Reclaimed, result.Skipped)

	// The Ready issue (#N) must be dispatched as Task-N with no manual call.
	wantDispatch := os.Getenv("QUEUE_DRAIN_SMOKE_EXPECT_DISPATCH") // e.g. Task-0001
	if wantDispatch != "" {
		found := false
		for _, taskID := range result.Dispatched {
			if taskID == wantDispatch {
				found = true
			}
		}
		if !found {
			t.Fatalf("expected live dispatch of %q, got dispatched=%v", wantDispatch, result.Dispatched)
		}
	}
	// The Never issue must NOT be dispatched.
	wantIgnore := os.Getenv("QUEUE_DRAIN_SMOKE_EXPECT_IGNORE") // e.g. Task-0002
	if wantIgnore != "" {
		for _, taskID := range result.Dispatched {
			if taskID == wantIgnore {
				t.Fatalf("Queue=Never issue %q must NOT be dispatched, got dispatched=%v", wantIgnore, result.Dispatched)
			}
		}
	}
}
