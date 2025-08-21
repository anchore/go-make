package github

import (
	"archive/zip"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"path"
	"path/filepath"
	"strings"

	"github.com/bmatcuk/doublestar/v4"

	"github.com/anchore/go-make/color"
	"github.com/anchore/go-make/config"
	"github.com/anchore/go-make/fetch"
	"github.com/anchore/go-make/file"
	"github.com/anchore/go-make/lang"
	"github.com/anchore/go-make/log"
	"github.com/anchore/go-make/script"
)

type Option func(Api)

type Api struct {
	// Token the Authorization: Bearer token to use, defaults to environment variable: GITHUB_TOKEN or `gh auth token`
	Token string

	// BaseURL base URL to the GitHub API defaults to https://api.github.com
	BaseURL string

	// Owner is the GitHub organization or user
	Owner string

	// Repo is the GitHub repository
	Repo string

	// Actor is the actor executing the workflow in GitHub, the login, e.g. kzantow
	Actor string
}

func NewClient(param ...Param) Api {
	token := config.Env("GITHUB_TOKEN", "")
	if token == "" {
		p := Payload()
		token = p.Token
		if token == "" {
			// run.Command will not install from binny, this will always use the provided `gh` command on the runner
			// token = lang.Continue(run.Command("gh", run.Args("auth", "token")))
			token = script.Run("gh auth token")
		}
	}
	a := Api{
		Token:   token,
		BaseURL: config.Env("GITHUB_API_URL", "https://api.github.com"),
		Owner:   config.Env("GITHUB_OWNER", ""),
		Repo:    config.Env("GITHUB_REPO", ""),
	}
	for _, p := range param {
		switch parts := strings.Split(p(), "="); parts[0] {
		case "owner":
			a.Owner = parts[1]
		case "repo":
			a.Repo = parts[1]
		case "actor":
			a.Actor = parts[1]
		}
	}
	return a
}

// ListWorkflowRuns returns the workflow runs for the given criteria
func (a Api) ListWorkflowRuns(ref Param, params ...Param) []WorkflowRun {
	// branch=${branch}&status=success&per_page=1&sort=created&direction=desc
	params = append(params, ref, sort("created"), direction("desc"))
	return get[WorkflowRunList](a, "/repos/%s/%s/actions/runs?%s", a.Owner, a.Repo, join(params, "&")).WorkflowRuns
}

// LatestWorkflowRun returns the latest workflow run for the given criteria
func (a Api) LatestWorkflowRun(ref Param, workflowName string, params ...Param) WorkflowRun {
	runs := a.ListWorkflowRuns(ref, params...)
	for _, run := range runs {
		switch run.Status {
		case "cancelled", "queued":
			continue
		}
		if nameMatches(workflowName, run.Name) {
			return run
		}
	}
	return WorkflowRun{}
}

func (a Api) ListArtifactsForWorkflow(ref Param, workflowName string, artifactName string, params ...Param) []Artifact {
	latestRun := a.LatestWorkflowRun(ref, workflowName, params...)
	if latestRun.ID == 0 {
		log.Log(color.Yellow("no workflow run found for %s/%s workflow: %s", a.Owner, a.Repo, workflowName))
		return nil
	}

	var out []Artifact
	artifacts := a.listWorkflowRunArtifacts(a.Owner, a.Repo, latestRun.ID, artifactName, params...)
	for _, artifact := range artifacts {
		if nameMatches(artifactName, artifact.Name) {
			out = append(out, artifact)
		}
	}
	return out
}

func (a Api) listWorkflowRunArtifacts(owner, repo string, runID int64, artifactName string, params ...Param) []Artifact {
	if isExactMatch(artifactName) {
		params = append(params, Name(artifactName))
	}
	return get[ArtifactList](a, fmt.Sprintf("/repos/%s/%s/actions/runs/%v/artifacts?%s", owner, repo, runID, join(params, "&"))).Artifacts
}

// DownloadArtifactDir downloads an artifact and extracts it to the targetDir
func (a Api) DownloadArtifactDir(ref Param, workflowName, artifactName, targetDir string, params ...Param) {
	targetDir = lang.Return(filepath.Abs(targetDir))

	artifacts := a.ListArtifactsForWorkflow(ref, workflowName, artifactName, params...)
	for _, artifact := range artifacts {
		file.EnsureDir(targetDir)
		a.extractArtifact(artifact.ID, targetDir)
	}
}

func (a Api) extractArtifact(artifactID int64, targetDir string) {
	f := lang.Return(os.CreateTemp(targetDir, ".gh-download-"))
	tempZip := filepath.Join(targetDir, f.Name())
	defer lang.Close(f, tempZip)

	// https://docs.github.com/en/rest/actions/artifacts?apiVersion=2022-11-28#download-an-artifact
	// https://api.github.com/repos/OWNER/REPO/actions/artifacts/ARTIFACT_ID/ARCHIVE_FORMAT
	// archive_format must be zip
	lang.Return(fetch.Fetch(fmt.Sprintf(a.BaseURL+"/repos/%s/%s/actions/artifacts/%v/zip", a.Owner, a.Repo, artifactID), a.headers(), fetch.Writer(f)))
	file.LogWorkdir()
	s := lang.Return(f.Stat())
	zipReader := lang.Return(zip.NewReader(f, s.Size()))
	for _, entry := range zipReader.File {
		extractArtifactEntry(entry, targetDir)
	}
}

func extractArtifactEntry(entry *zip.File, targetDir string) {
	if entry == nil || entry.FileInfo() == nil || entry.FileInfo().IsDir() {
		return
	}
	if filepath.IsAbs(entry.Name) {
		log.Debug("got absolute path to zip entry: %v", entry.Name)
	}

	targetFile := filepath.Join(targetDir, filepath.Join(path.Split(entry.Name))) //nolint:gocritic
	targetFile = lang.Return(filepath.Abs(targetFile))
	if !strings.HasPrefix(targetFile, targetDir) {
		log.Log("skipping zip entry outside of target dir: %v abs: %v", entry.Name, targetFile)
		return
	}
	file.EnsureDir(filepath.Dir(targetFile))

	target := lang.Return(os.OpenFile(targetFile, os.O_CREATE|os.O_TRUNC|os.O_WRONLY, entry.FileInfo().Mode()))
	defer lang.Close(target, targetFile)
	rdr := lang.Return(entry.Open())
	defer lang.Close(rdr, targetFile)
	lang.Return(io.Copy(target, rdr))
}

func (a Api) headers() fetch.Option {
	return fetch.Headers(map[string]string{
		"Authorization":        fmt.Sprintf("Bearer %s", a.Token),
		"Accept":               "application/vnd.github+json", // or application/vnd.github.v3+json?
		"X-GitHub-Api-Version": "2022-11-28",
	})
}

func get[T any](a Api, url string, args ...any) T {
	contents := lang.Return(fetch.Fetch(a.BaseURL+fmt.Sprintf(url, args...), a.headers()))
	if config.Debug {
		log.Debug("fetch result %v: %v", url, log.FormatJSON(contents))
	}
	var out T
	lang.Throw(json.NewDecoder(strings.NewReader(contents)).Decode(&out))
	return out
}

func join(params []Param, joiner string) string {
	out := ""
	for i, param := range params {
		if i > 0 {
			out += joiner
		}
		out += param()
	}
	return out
}

func queryParam(param, value string) Param {
	return func() string { return param + "=" + value }
}

// nameMatches returns true if the name matches the expression, lowercase matching, supports globbing
func nameMatches(pattern, name string) bool {
	name = strings.TrimSpace(name)
	name = strings.ToLower(name)
	pattern = strings.ToLower(pattern)
	return lang.Return(doublestar.Match(pattern, name))
}

func isExactMatch(pattern string) bool {
	return !strings.ContainsAny(pattern, "*?[{")
}
