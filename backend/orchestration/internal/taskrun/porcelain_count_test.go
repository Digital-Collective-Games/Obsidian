package taskrun

import "testing"

// Landing 2 Step 1: a stale PRUNABLE git worktree must NOT count as an occupied slot
// (it blocked dispatch in the Landing-1 regression), and only lanes UNDER the owned-lane
// root count (not the main checkout).
func TestCountOwnedLaneWorktreesFromPorcelainExcludesPrunableAndOutside(t *testing.T) {
	laneRoot := `C:\Temp\cdxow`
	porcelain := "" +
		"worktree C:/Agent/QueueDrainTestbed2\n" + // main checkout, OUTSIDE the lane root
		"HEAD 1111111111111111111111111111111111111111\n" +
		"branch refs/heads/main\n" +
		"\n" +
		"worktree C:/Temp/cdxow/Task-0001-aaaa/w\n" + // healthy lane -> counts
		"HEAD 2222222222222222222222222222222222222222\n" +
		"detached\n" +
		"\n" +
		"worktree C:/Temp/cdxow/Task-0002-bbbb/w\n" + // STALE prunable lane -> must NOT count
		"HEAD 3333333333333333333333333333333333333333\n" +
		"detached\n" +
		"prunable gitdir file points to non-existent location\n" +
		"\n"

	got := countOwnedLaneWorktreesFromPorcelain([]byte(porcelain), laneRoot)
	if got != 1 {
		t.Fatalf("count = %d, want 1 (only the healthy under-lane worktree; prunable + outside-lane excluded)", got)
	}

	// A bare `prunable` line (no reason) must also be excluded.
	bare := "worktree C:/Temp/cdxow/Task-0003-cccc/w\nHEAD 4444444444444444444444444444444444444444\ndetached\nprunable\n\n"
	if got := countOwnedLaneWorktreesFromPorcelain([]byte(bare), laneRoot); got != 0 {
		t.Fatalf("bare-prunable count = %d, want 0", got)
	}
}
