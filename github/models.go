package github

import (
	"strings"
	"time"

	"github.com/anchore/go-make/log"
)

type ArtifactWorkflowRun struct {
	ID         int64  `json:"id,omitempty"`
	HeadBranch string `json:"head_branch,omitempty"`
	HeadSHA    string `json:"head_sha,omitempty"`
}

type Artifact struct {
	ID                 int64               `json:"id,omitempty"`
	NodeID             string              `json:"node_id,omitempty"`
	Name               string              `json:"name,omitempty"`
	SizeInBytes        int64               `json:"size_in_bytes,omitempty"`
	URL                string              `json:"url,omitempty"`
	ArchiveDownloadURL string              `json:"archive_download_url,omitempty"`
	Expired            bool                `json:"expired,omitempty"`
	CreatedAt          Timestamp           `json:"created_at,omitempty"`
	UpdatedAt          Timestamp           `json:"updated_at,omitempty"`
	ExpiresAt          Timestamp           `json:"expires_at,omitempty"`
	Digest             string              `json:"digest,omitempty"`
	WorkflowRun        ArtifactWorkflowRun `json:"workflow_run,omitempty"`
}

type ArtifactList struct {
	TotalCount int64      `json:"total_count,omitempty"`
	Artifacts  []Artifact `json:"artifacts,omitempty"`
}

type WorkflowRun struct {
	ID                 int64         `json:"id,omitempty"`
	Name               string        `json:"name,omitempty"`
	NodeID             string        `json:"node_id,omitempty"`
	HeadBranch         string        `json:"head_branch,omitempty"`
	HeadSHA            string        `json:"head_sha,omitempty"`
	Path               string        `json:"path,omitempty"`
	RunNumber          int           `json:"run_number,omitempty"`
	RunAttempt         int           `json:"run_attempt,omitempty"`
	Event              string        `json:"event,omitempty"`
	DisplayTitle       string        `json:"display_title,omitempty"`
	Status             string        `json:"status,omitempty"`
	Conclusion         string        `json:"conclusion,omitempty"` // success or failure
	WorkflowID         int64         `json:"workflow_id,omitempty"`
	CheckSuiteID       int64         `json:"check_suite_id,omitempty"`
	CheckSuiteNodeID   string        `json:"check_suite_node_id,omitempty"`
	URL                string        `json:"url,omitempty"`
	HTMLURL            string        `json:"html_url,omitempty"`
	PullRequests       []PullRequest `json:"pull_requests,omitempty"`
	CreatedAt          Timestamp     `json:"created_at,omitempty"`
	UpdatedAt          Timestamp     `json:"updated_at,omitempty"`
	RunStartedAt       Timestamp     `json:"run_started_at,omitempty"`
	PreviousAttemptURL string        `json:"previous_attempt_url,omitempty"`
	HeadCommit         HeadCommit    `json:"head_commit,omitempty"`
	Repository         Repository    `json:"repository,omitempty"`
	HeadRepository     Repository    `json:"head_repository,omitempty"`
	Actor              User          `json:"actor,omitempty"`
	TriggeringActor    User          `json:"triggering_actor,omitempty"`
}

type PullRequest struct {
	URL    string `json:"url,omitempty"`
	Number int    `json:"number,omitempty"`
	Head   Ref    `json:"head,omitempty"`
}

type Ref struct {
	Ref string `json:"ref,omitempty"`
	SHA string `json:"sha,omitempty"`
}

type HeadCommit struct {
	ID string `json:"id,omitempty"`
}

type Repository struct {
	ID          int64  `json:"id"`
	NodeID      string `json:"node_id"`
	Name        string `json:"name"`
	FullName    string `json:"full_name"`
	Private     bool   `json:"private"`
	Owner       User   `json:"owner"`
	HTMLURL     string `json:"html_url"`
	Description string `json:"description"`
	Fork        bool   `json:"fork"`
	URL         string `json:"url"`
}

type Timestamp time.Time

func (t *Timestamp) UnmarshalJSON(data []byte) (err error) {
	str := string(data)
	str = strings.Trim(str, `"'`)
	var ts time.Time
	ts, err = time.Parse(time.RFC3339, str)
	if err != nil {
		log.Error(err)
	} else {
		*t = Timestamp(ts)
	}
	return nil
}

type WorkflowRunList struct {
	TotalCount   int64         `json:"total_count,omitempty"`
	WorkflowRuns []WorkflowRun `json:"workflow_runs,omitempty"`
}

type User struct {
	Type    string `json:"type,omitempty"`
	Login   string `json:"login,omitempty"`
	ID      int64  `json:"id,omitempty"`
	URL     string `json:"url,omitempty"`
	HtmlURL string `json:"html_url,omitempty"`
}
