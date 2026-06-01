package queue

import (
	"fmt"
	"strings"
	"testing"
)

// TestGitHubQueueProviderParsesGhReads proves the production provider turns the
// gh CLI reads (org issue-fields, issue list, per-issue field values) into the
// IssueState the consumer reads — WITHOUT any live GitHub access (the gh runner
// is injected). It also confirms the provider issues only READ calls (no writes).
func TestGitHubQueueProviderParsesGhReads(t *testing.T) {
	var calls []string
	fakeRun := func(args ...string) ([]byte, error) {
		joined := strings.Join(args, " ")
		calls = append(calls, joined)
		switch {
		case strings.Contains(joined, "/issue-fields"):
			return []byte(`[{"id":42656828,"name":"Queue"},{"id":42656829,"name":"Human Needed"}]`), nil
		case strings.HasPrefix(joined, "issue list"):
			return []byte(`[{"number":7001,"state":"OPEN"},{"number":7002,"state":"OPEN"},{"number":7003,"state":"CLOSED"}]`), nil
		case strings.Contains(joined, "/issues/7001/issue-field-values"):
			return []byte(`[{"issue_field_id":42656828,"single_select_option":{"name":"Ready"}}]`), nil
		case strings.Contains(joined, "/issues/7002/issue-field-values"):
			return []byte(`[{"issue_field_id":42656828,"single_select_option":{"name":"Never"}},{"issue_field_id":42656829,"single_select_option":{"name":"Yes"}}]`), nil
		case strings.Contains(joined, "/issues/7003/issue-field-values"):
			return []byte(`[]`), nil
		default:
			return nil, fmt.Errorf("unexpected gh call: %s", joined)
		}
	}

	provider := &ghQueueProvider{owner: "Digital-Collective-Games", repo: testRepo, limit: 200, run: fakeRun}
	issues, err := provider.ListReadyIssues(testRepo)
	if err != nil {
		t.Fatalf("ListReadyIssues: %v", err)
	}
	if len(issues) != 3 {
		t.Fatalf("got %d issues, want 3", len(issues))
	}

	byNum := map[int]IssueState{}
	for _, issue := range issues {
		byNum[issue.Number] = issue.State
	}
	if s := byNum[7001]; s.Queue != QueueReady || s.HumanNeeded || s.Closed {
		t.Fatalf("#7001 state = %#v, want Queue=Ready open not-human-needed", s)
	}
	if s := byNum[7002]; s.Queue != QueueNever || !s.HumanNeeded {
		t.Fatalf("#7002 state = %#v, want Queue=Never HumanNeeded=true", s)
	}
	if s := byNum[7003]; !s.Closed {
		t.Fatalf("#7003 state = %#v, want Closed=true", s)
	}

	// Every gh call must be a READ (api GET or issue list/view) — never a write
	// (no -X POST/PATCH/PUT/DELETE, no `issue edit`), proving the consumer is
	// read-only against GitHub (A4.6).
	for _, c := range calls {
		if strings.Contains(c, "-X POST") || strings.Contains(c, "-X PATCH") ||
			strings.Contains(c, "-X PUT") || strings.Contains(c, "-X DELETE") ||
			strings.Contains(c, "issue edit") || strings.Contains(c, "issue close") {
			t.Fatalf("provider issued a non-read gh call: %q", c)
		}
	}
}

// The end-to-end provider->consumer path with the fake gh runner: #7001 (Ready)
// dispatches, #7002 (Never+HumanNeeded) parks-not-dispatches, #7003 (closed) has
// no owned lane so it is skipped — proving the gh-parsed state drives the same
// DecideQueueAction loop the deterministic consumer tests exercise.
func TestGitHubProviderDrivesConsumerLoop(t *testing.T) {
	fakeRun := func(args ...string) ([]byte, error) {
		joined := strings.Join(args, " ")
		switch {
		case strings.Contains(joined, "/issue-fields"):
			return []byte(`[{"id":1,"name":"Queue"},{"id":2,"name":"Human Needed"}]`), nil
		case strings.HasPrefix(joined, "issue list"):
			return []byte(`[{"number":7001,"state":"OPEN"},{"number":7002,"state":"OPEN"},{"number":7003,"state":"CLOSED"}]`), nil
		case strings.Contains(joined, "/issues/7001/"):
			return []byte(`[{"issue_field_id":1,"single_select_option":{"name":"Ready"}}]`), nil
		case strings.Contains(joined, "/issues/7002/"):
			return []byte(`[{"issue_field_id":2,"single_select_option":{"name":"Yes"}}]`), nil
		case strings.Contains(joined, "/issues/7003/"):
			return []byte(`[]`), nil
		default:
			return nil, fmt.Errorf("unexpected gh call: %s", joined)
		}
	}
	provider := &ghQueueProvider{owner: "Digital-Collective-Games", repo: testRepo, limit: 200, run: fakeRun}
	dispatcher := newFakeDispatcher("Task-7002") // #7002 parked task already owns a lane
	consumer := NewConsumer(testRepo, provider, dispatcher, fixedIdleSizer(4))

	result, err := consumer.DrainOnce(nil)
	if err != nil {
		t.Fatalf("DrainOnce: %v", err)
	}
	if len(result.Dispatched) != 1 || result.Dispatched[0] != "Task-7001" {
		t.Fatalf("dispatched = %v, want [Task-7001]", result.Dispatched)
	}
	if state := dispatcher.parked["Task-7002"]; state != runGateStateParkedAwaitingClosure {
		t.Fatalf("Task-7002 parked = %q, want awaiting closure", state)
	}
}
