package queue

import (
	"bytes"
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"strconv"
	"strings"
)

// QueueProvider reads the GitHub-issue queue for a repo. The O3 consumer is
// READ-ONLY against GitHub: it never writes issue state (the only GitHub-write
// path stays the obsidian-operator skill surface, A4.6). The provider returns
// every open issue's observed state plus closed issues that still own a worktree,
// so the consumer can run DecideQueueAction over each one (dispatch Ready, park
// Human Needed=Yes, reclaim on close).
//
// It is an interface so the consumer is unit-testable with a FAKE provider: a
// test supplies a deterministic issue set and asserts the consumer dispatches,
// parks, or reclaims without any live GitHub access.
type QueueProvider interface {
	// ListReadyIssues returns the issues the consumer must evaluate for the repo.
	// It returns at least every OPEN issue with its Queue / Human Needed state; an
	// implementation MAY also include closed issues that still hold a worktree so
	// the consumer can reclaim them. The consumer applies DecideQueueAction to each.
	ListReadyIssues(repo string) ([]IssueRef, error)
	// CloseIssue closes issue #number on repo, exactly as a human closing the issue
	// would (reason "completed"). It is the ONLY GitHub-write the consumer performs,
	// and it is invoked ONLY when the TEST-ONLY OBSIDIAN_AUTO_CLOSE_QUEUED auto-close
	// is enabled (otherwise the consumer never calls it and stays read-only, A4.6).
	CloseIssue(repo string, number int) error
	// DequeueIssue is the provider WRITE that takes a task out of the queue: it sets the
	// issue's Queue single-select to Never (QueueNever), the same field ListReadyIssues
	// polls for Ready. It is the symmetric sibling of the Queue read and of CloseIssue.
	// It NEVER closes the issue (the issue stays open, preserving human-only closure) and
	// is idempotent — setting Never when it is already Never is a no-op. Task-0016 Eject
	// and the standalone dequeue endpoint both call it; it is the FIRST task-provider
	// queue write (the consumer's own poll path stays read-only).
	DequeueIssue(repo string, number int) error
}

// IssueRef is one provider-observed GitHub issue. Number is the issue #N (which
// maps 1:1 to Tracking/Task-N, SKILL.md provider contract). State carries the
// fields DecideQueueAction reads.
type IssueRef struct {
	// Number is the GitHub issue number (#N).
	Number int
	// State is the observed issue state (closed / Human Needed / Queue).
	State IssueState
}

// ghQueueProvider is the production provider. It is a thin wrapper over the
// already-authenticated `gh` CLI, mirroring the reads the obsidian-operator
// reconcile script performs (org issue-fields for the field-id map, gh issue list
// for open issues, issue-field-values per issue). It performs only READS.
type ghQueueProvider struct {
	// owner is the org/owner that owns the issue fields (e.g. Digital-Collective-Games).
	owner string
	// repo is the full provider repo (e.g. Digital-Collective-Games/Obsidian).
	repo string
	// limit caps how many issues gh enumerates per poll.
	limit int
	// run executes a gh argv and returns stdout; injectable for tests. Production
	// uses runGh (os/exec "gh ...").
	run func(args ...string) ([]byte, error)
}

// NewGitHubQueueProvider builds the production read-only gh-backed provider for a
// provider repo (owner/name). limit<=0 falls back to a sane default page size.
func NewGitHubQueueProvider(repo string, limit int) (QueueProvider, error) {
	owner, _, ok := splitOwnerRepo(repo)
	if !ok {
		return nil, fmt.Errorf("provider repo %q is not in owner/name form", repo)
	}
	if limit <= 0 {
		limit = 200
	}
	return &ghQueueProvider{owner: owner, repo: repo, limit: limit, run: runGh}, nil
}

func splitOwnerRepo(repo string) (owner string, name string, ok bool) {
	parts := strings.SplitN(strings.TrimSpace(repo), "/", 2)
	if len(parts) != 2 || parts[0] == "" || parts[1] == "" {
		return "", "", false
	}
	return parts[0], parts[1], true
}

func runGh(args ...string) ([]byte, error) {
	cmd := exec.Command("gh", args...)
	var stdout, stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr
	if err := cmd.Run(); err != nil {
		return nil, fmt.Errorf("gh %s: %w: %s", strings.Join(args, " "), err, strings.TrimSpace(stderr.String()))
	}
	return stdout.Bytes(), nil
}

type ghIssueField struct {
	ID       int    `json:"id"`
	Name     string `json:"name"`
	DataType string `json:"data_type"`
	Options  []struct {
		ID   int    `json:"id"`
		Name string `json:"name"`
	} `json:"options"`
}

type ghIssueListEntry struct {
	Number int    `json:"number"`
	State  string `json:"state"`
}

type ghFieldValue struct {
	IssueFieldID int `json:"issue_field_id"`
	// Value is the raw field value. For single_select fields GitHub returns the
	// option id here (a number) and the human-readable name in single_select_option;
	// for text/other fields it may be a string. It is kept raw so a numeric option
	// id never fails to decode, and only used as a string fallback when there is no
	// single_select_option.
	Value              json.RawMessage `json:"value"`
	SingleSelectOption *struct {
		Name string `json:"name"`
	} `json:"single_select_option"`
}

// ListReadyIssues reads every issue's observed state via gh (READ ONLY). It
// resolves the org field-id map once, lists issues, and reads each issue's
// Queue / Human Needed field values, returning an IssueRef per issue.
func (p *ghQueueProvider) ListReadyIssues(repo string) ([]IssueRef, error) {
	if repo == "" {
		repo = p.repo
	}
	fieldIDByName, err := p.fieldIDMap()
	if err != nil {
		return nil, err
	}
	issues, err := p.listIssues(repo)
	if err != nil {
		return nil, err
	}
	refs := make([]IssueRef, 0, len(issues))
	for _, issue := range issues {
		state := IssueState{Closed: strings.EqualFold(issue.State, "closed")}
		values, err := p.fieldValues(repo, issue.Number)
		if err != nil {
			return nil, err
		}
		for _, value := range values {
			name := fieldIDByName[value.IssueFieldID]
			optionName := ""
			if value.SingleSelectOption != nil {
				optionName = value.SingleSelectOption.Name
			} else if len(value.Value) > 0 {
				// Fallback for non-single-select fields: decode the raw value as a
				// string when it is one; a numeric id without an option name is ignored.
				var s string
				if err := json.Unmarshal(value.Value, &s); err == nil {
					optionName = s
				}
			}
			switch name {
			case "Queue":
				state.Queue = QueueFieldValue(optionName)
			case "Human Needed":
				state.HumanNeeded = strings.EqualFold(optionName, "Yes")
			}
		}
		refs = append(refs, IssueRef{Number: issue.Number, State: state})
	}
	return refs, nil
}

// CloseIssue closes issue #number on repo via the gh CLI, the same close a human
// performs. It is the ONLY write the provider issues and runs ONLY behind the
// TEST-ONLY auto-close flag. The gh CLI accepts the bare issue number.
func (p *ghQueueProvider) CloseIssue(repo string, number int) error {
	if repo == "" {
		repo = p.repo
	}
	_, err := p.run("issue", "close", strconv.Itoa(number), "--repo", repo, "--reason", "completed")
	return err
}

// DequeueIssue sets the issue's Queue single-select to Never via the gh CLI — the
// symmetric WRITE counterpart of the Queue read. It resolves the org "Queue" field id
// (and verifies the single-select carries the Never option, reusing the QueueNever
// constant) and POSTs the issue-field-value, the same write the obsidian-operator sync
// performs (POST /repos/<repo>/issues/<n>/issue-field-values with
// {issue_field_values:[{field_id, value:"Never"}]}). The body is passed through the
// injectable run func via --input so a test never touches real GitHub. It never closes
// the issue; setting Never when already Never is a server-side no-op (idempotent).
func (p *ghQueueProvider) DequeueIssue(repo string, number int) error {
	if repo == "" {
		repo = p.repo
	}
	field, err := p.queueField()
	if err != nil {
		return err
	}
	body, err := json.Marshal(map[string]any{
		"issue_field_values": []map[string]any{
			{"field_id": field.ID, "value": string(QueueNever)},
		},
	})
	if err != nil {
		return fmt.Errorf("encode dequeue body: %w", err)
	}
	tmp, err := os.CreateTemp("", "dequeue-*.json")
	if err != nil {
		return fmt.Errorf("create dequeue body temp: %w", err)
	}
	defer os.Remove(tmp.Name())
	if _, err := tmp.Write(body); err != nil {
		tmp.Close()
		return fmt.Errorf("write dequeue body temp: %w", err)
	}
	if err := tmp.Close(); err != nil {
		return fmt.Errorf("close dequeue body temp: %w", err)
	}
	_, err = p.run("api", "-X", "POST", fmt.Sprintf("/repos/%s/issues/%d/issue-field-values", repo, number), "--input", tmp.Name())
	return err
}

// queueField resolves the org "Queue" single-select issue field, verifying it carries
// the Never option so the dequeue write targets a real value.
func (p *ghQueueProvider) queueField() (ghIssueField, error) {
	raw, err := p.run("api", "/orgs/"+p.owner+"/issue-fields")
	if err != nil {
		return ghIssueField{}, err
	}
	var fields []ghIssueField
	if err := json.Unmarshal(raw, &fields); err != nil {
		return ghIssueField{}, fmt.Errorf("decode org issue-fields: %w", err)
	}
	for _, field := range fields {
		if field.Name != "Queue" {
			continue
		}
		if field.DataType != "" && field.DataType != "single_select" {
			return ghIssueField{}, fmt.Errorf("issue field %q must be single_select, got %q", field.Name, field.DataType)
		}
		hasNever := len(field.Options) == 0 // tolerate a fields read that omits options
		for _, opt := range field.Options {
			if opt.Name == string(QueueNever) {
				hasNever = true
				break
			}
		}
		if !hasNever {
			return ghIssueField{}, fmt.Errorf("issue field %q has no %q option", field.Name, QueueNever)
		}
		return field, nil
	}
	return ghIssueField{}, fmt.Errorf("issue field %q not found for org %q", "Queue", p.owner)
}

func (p *ghQueueProvider) fieldIDMap() (map[int]string, error) {
	raw, err := p.run("api", "/orgs/"+p.owner+"/issue-fields")
	if err != nil {
		return nil, err
	}
	var fields []ghIssueField
	if err := json.Unmarshal(raw, &fields); err != nil {
		return nil, fmt.Errorf("decode org issue-fields: %w", err)
	}
	out := make(map[int]string, len(fields))
	for _, field := range fields {
		out[field.ID] = field.Name
	}
	return out, nil
}

func (p *ghQueueProvider) listIssues(repo string) ([]ghIssueListEntry, error) {
	raw, err := p.run("issue", "list", "--repo", repo, "--state", "all", "--limit", strconv.Itoa(p.limit), "--json", "number,state")
	if err != nil {
		return nil, err
	}
	var entries []ghIssueListEntry
	if err := json.Unmarshal(raw, &entries); err != nil {
		return nil, fmt.Errorf("decode issue list: %w", err)
	}
	return entries, nil
}

func (p *ghQueueProvider) fieldValues(repo string, number int) ([]ghFieldValue, error) {
	raw, err := p.run("api", fmt.Sprintf("/repos/%s/issues/%d/issue-field-values", repo, number))
	if err != nil {
		return nil, err
	}
	var values []ghFieldValue
	if err := json.Unmarshal(raw, &values); err != nil {
		return nil, fmt.Errorf("decode issue-field-values for #%d: %w", number, err)
	}
	return values, nil
}
