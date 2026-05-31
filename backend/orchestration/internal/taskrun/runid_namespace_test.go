package taskrun

import "testing"

// BUG-0003 Step 1: the empty-namespace run id MUST be byte-identical to the historical
// ActiveRunID (the single-repo control plane and existing call sites depend on it), and
// two different repo namespaces MUST produce distinct run ids for the same task (the
// cross-repo collision fix).
func TestActiveRunIDForRepoShimAndDistinctness(t *testing.T) {
	if got, want := ActiveRunIDForRepo("", "Task-0001"), ActiveRunID("Task-0001"); got != want {
		t.Fatalf("empty-namespace run id = %q, want shim-equivalent %q", got, want)
	}
	if got, want := ActiveRunIDForRepo("", "Task-0001"), "taskrun--Task-0001--active"; got != want {
		t.Fatalf("legacy run id = %q, want %q", got, want)
	}

	a := ActiveRunIDForRepo("RepoA", "Task-0001")
	b := ActiveRunIDForRepo("RepoB", "Task-0001")
	if a == b {
		t.Fatalf("two repos' Task-0001 collided on run id: %q", a)
	}
	if a == ActiveRunID("Task-0001") || b == ActiveRunID("Task-0001") {
		t.Fatalf("namespaced run id collided with the legacy global id (a=%q b=%q legacy=%q)", a, b, ActiveRunID("Task-0001"))
	}
	if want := "taskrun--RepoA--Task-0001--active"; a != want {
		t.Fatalf("namespaced run id = %q, want %q", a, want)
	}
}
