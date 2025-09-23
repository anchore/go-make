package github

import (
	"archive/zip"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"path"
	"path/filepath"
	"strings"
	"sync"

	"github.com/bmatcuk/doublestar/v4"

	. "github.com/anchore/go-make"
	"github.com/anchore/go-make/config"
	"github.com/anchore/go-make/fetch"
	"github.com/anchore/go-make/file"
	"github.com/anchore/go-make/git"
	"github.com/anchore/go-make/lang"
	"github.com/anchore/go-make/log"
)

type Option func(Api)

type Api struct {
	// Token the Authorization: Bearer token to use, defaults to environment variable: GITHUB_TOKEN or `gh auth token`
	Token string

	// BaseURL base URL to the GitHub API defaults to https://api.github.com
	BaseURL string

	// Repo is the full GitHub repository, e.g. anchore/syft
	Repo string

	// Actor is the actor executing the workflow in GitHub, the login, e.g. kzantow
	Actor string
}

// NewClient creates a new GitHub API client, looking up information automatically from the environment
// when run in a GitHub Actions runner
func NewClient(params ...Param) Api {
	p := Payload() // look up payload when running on github runners

	log.Debug("NewClient: %v", map[string]string{
		"git.Revision": git.Revision(),
		"payload":      log.FormatJSON(string(lang.Continue(json.Marshal(p)))),
	})

	if p.Token == "" {
		// try to get locally authenticated token
		p.Token = Run("gh auth token")
	}

	a := Api{
		Token:   p.Token,
		BaseURL: lang.Default(p.ApiURL, "https://api.github.com"),
		Repo:    p.Repo,
	}

	for _, param := range params {
		switch param.name {
		case "repo":
			a.Repo = param.value
		case "actor":
			a.Actor = param.value
		case "token":
			a.Token = param.value
		}
	}

	return a
}

func (a Api) DeleteArtifact(artifactID int64) error {
	return fetch.Delete(a.BaseURL+fmt.Sprintf("/repos/%s/actions/artifacts/%v", a.Repo, artifactID), a.headers())
}

// ListWorkflowRuns returns the workflow runs for the given criteria
func (a Api) listWorkflowRuns(branch string, params ...Param) ([]WorkflowRun, error) {
	// branch=${branch}&status=success&per_page=1&sort=created&direction=desc
	params = append(params, Branch(branch), sort("created"), direction("desc"))
	runs, err := get[WorkflowRunList](a, "/repos/%s/actions/runs?%s", a.Repo, join(params, "&"))
	return runs.WorkflowRuns, err
}

// LatestWorkflowRun returns the latest completed workflow run for a branch
func (a Api) LatestWorkflowRun(branch, workflowNameGlob string) (WorkflowRun, error) {
	runs, err := a.listWorkflowRuns(branch, PerPage(100), Status("success"))
	if err != nil {
		return WorkflowRun{}, err
	}
	for _, run := range runs {
		switch run.Status {
		case "cancelled", "queued":
			continue
		}
		if workflowNameGlob == "" || nameMatches(workflowNameGlob, run.Name) {
			return run, nil
		}
	}
	return WorkflowRun{}, fmt.Errorf("no workflow run found for %s workflow: %s", a.Repo, workflowNameGlob)
}

// ListArtifactsForBranch returns artifacts for the latest run on the given workflow
func (a Api) ListArtifactsForBranch(branch, workflowNameGlob, artifactNameGlob string) ([]Artifact, error) {
	latestRun, err := a.LatestWorkflowRun(branch, workflowNameGlob)
	if err != nil {
		return nil, err
	}
	if latestRun.ID == 0 {
		return nil, fmt.Errorf("no workflow run found for %s workflow: %s", a.Repo, workflowNameGlob)
	}
	return a.ListArtifactsForWorkflowRun(latestRun.ID, artifactNameGlob)
}

func (a Api) ListArtifactsForWorkflowRun(runID int64, artifactNameGlob string) ([]Artifact, error) {
	return a.listWorkflowRunArtifacts(a.Repo, runID, artifactNameGlob)
}

func (a Api) listWorkflowRunArtifacts(repo string, runID int64, artifactNameGlob string) ([]Artifact, error) {
	rsp, err := get[ArtifactList](a, "/repos/%s/actions/runs/%v/artifacts", repo, runID)
	if err != nil {
		return nil, err
	}
	var out []Artifact
	for _, artifact := range rsp.Artifacts {
		if artifactNameGlob == "" || nameMatches(artifactNameGlob, artifact.Name) {
			out = append(out, artifact)
		}
	}
	return out, nil
}

// DownloadBranchArtifactDir downloads an artifact from the latest run on a branch and extracts it to the targetDir
func (a Api) DownloadBranchArtifactDir(branch, workflowName, artifactName, targetDir string) error {
	latestRun, err := a.LatestWorkflowRun(branch, workflowName)
	if err != nil {
		return err
	}
	if latestRun.ID == 0 {
		return fmt.Errorf("no workflow run found for %s workflow: %s", a.Repo, workflowName)
	}
	return a.DownloadArtifactDir(latestRun.ID, artifactName, targetDir)
}

// DownloadArtifactDir downloads an artifact and extracts it to the targetDir
func (a Api) DownloadArtifactDir(runID int64, artifactNameGlob, targetDir string) error {
	targetDir = lang.Return(filepath.Abs(targetDir))

	artifacts, err := a.ListArtifactsForWorkflowRun(runID, artifactNameGlob)
	if err != nil {
		return err
	}

	if err = ensureDir(targetDir); err != nil {
		return err
	}

	for _, artifact := range artifacts {
		if err = a.downloadAndExtractArtifact(artifact.ID, targetDir); err != nil {
			return err
		}
	}
	return nil
}

func (a Api) downloadAndExtractArtifact(artifactID int64, targetDir string) error {
	f := lang.Return(os.CreateTemp(targetDir, ".gh-download-"))
	tempZip := filepath.Join(targetDir, f.Name())
	defer lang.Close(f, tempZip)

	// https://docs.github.com/en/rest/actions/artifacts?apiVersion=2022-11-28#download-an-artifact
	// https://api.github.com/repos/OWNER/REPO/actions/artifacts/ARTIFACT_ID/ARCHIVE_FORMAT
	// archive_format must be zip
	_, err := fetch.Fetch(fmt.Sprintf(a.BaseURL+"/repos/%s/actions/artifacts/%v/zip", a.Repo, artifactID), a.headers(), fetch.Writer(f))
	if err != nil {
		return err
	}
	file.LogWorkdir()
	s, err := f.Stat()
	if err != nil {
		return err
	}
	zipReader := lang.Return(zip.NewReader(f, s.Size()))
	for _, entry := range zipReader.File {
		if err = extractArtifactEntry(entry, targetDir); err != nil {
			return err
		}
	}
	return nil
}

func extractArtifactEntry(entry *zip.File, targetDir string) error {
	if entry == nil || entry.FileInfo() == nil || entry.FileInfo().IsDir() {
		return nil
	}
	if filepath.IsAbs(entry.Name) {
		log.Debug("got absolute path to zip entry: %v", entry.Name)
	}

	targetFile := filepath.Join(targetDir, filepath.Join(path.Split(entry.Name))) //nolint:gocritic
	targetFile = lang.Return(filepath.Abs(targetFile))
	if !strings.HasPrefix(targetFile, targetDir) {
		log.Info("skipping zip entry outside of target dir: %v abs: %v", entry.Name, targetFile)
		return nil
	}

	if err := ensureDir(targetDir); err != nil {
		return err
	}

	target, err := os.OpenFile(targetFile, os.O_CREATE|os.O_TRUNC|os.O_WRONLY, entry.FileInfo().Mode())
	if err != nil {
		return err
	}

	defer lang.Close(target, targetFile)
	rdr, err := entry.Open()
	if err != nil {
		return err
	}
	defer lang.Close(rdr, targetFile)
	_, err = io.CopyN(target, rdr, 2*fetch.GB)
	if !errors.Is(err, io.EOF) {
		return err
	}
	return nil
}

func (a Api) headers() fetch.Option {
	return fetch.Headers(map[string]string{
		"Authorization":        fmt.Sprintf("Bearer %s", a.Token),
		"Accept":               "application/vnd.github+json", // or application/vnd.github.v3+json?
		"X-GitHub-Api-Version": "2022-11-28",
	})
}

func get[T any](a Api, url string, args ...any) (T, error) {
	var out T
	contents, err := fetch.Fetch(a.BaseURL+fmt.Sprintf(url, args...), a.headers())
	if err != nil {
		return out, err
	}
	if config.Debug {
		log.Debug("fetch result %v: %v", url, log.FormatJSON(contents))
	}
	err = json.NewDecoder(strings.NewReader(contents)).Decode(&out)
	return out, err
}

func join(params []Param, joiner string) string {
	out := ""
	for i, param := range params {
		if i > 0 {
			out += joiner
		}
		out += param.name + "=" + param.value
	}
	return out
}

// nameMatches returns true if the name matches the expression, lowercase matching, supports globbing
func nameMatches(pattern, name string) bool {
	name = strings.TrimSpace(name)
	name = strings.ToLower(name)
	pattern = strings.ToLower(pattern)
	return lang.Return(doublestar.Match(pattern, name))
}

func ensureDir(dir string) error {
	return lang.Catch(func() {
		file.EnsureDir(filepath.Dir(dir))
	})
}

var envFile = sync.OnceValue(func() map[string]string {
	p := filepath.Join(config.Env("HOME", config.Env("USERPROFILE", lang.Continue(os.UserHomeDir()))), ".bootstrap_actions_env")
	f, err := os.Open(p)
	if err != nil {
		if config.CI {
			log.Info("unable to read env file: %v: %v", p, err)
		} else {
			log.Debug("unable to read env file: %v: %v", p, err)
		}
	}
	out := map[string]string{}
	if f != nil {
		defer lang.Close(f, p)
		log.Error(json.NewDecoder(f).Decode(&out))
	}
	log.Debug("read env file %v: %v", p, out)
	return out
})

func authTokenFromEnvFile() string {
	return envFile()["GITHUB_TOKEN"]
}
