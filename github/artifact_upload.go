package github

import (
	"fmt"
	"os"
	"path/filepath"
	"strconv"
	"strings"

	"github.com/bmatcuk/doublestar/v4"

	. "github.com/anchore/go-make"
	"github.com/anchore/go-make/config"
	"github.com/anchore/go-make/file"
	"github.com/anchore/go-make/lang"
	"github.com/anchore/go-make/log"
	"github.com/anchore/go-make/node"
	"github.com/anchore/go-make/run"
	"github.com/anchore/go-make/template"
)

const (
	knownActionsArtifactVersion = "2.3.2"
)

type UploadArtifactOption struct {
	ArtifactName  string
	Overwrite     bool
	RetentionDays uint
	Glob          string
	Files         []string
}

// UploadArtifactDir will compress all files in the basedir into an artifact
// attached to the currently running workflow. Optionally limit the files by
// including ArtifactFiles or ArtifactGlob, which can be relative to the baseDir.
// To specify a custom name, use ArtifactName
func (a Api) UploadArtifactDir(baseDir string, opts UploadArtifactOption) (int64, error) {
	// Github actions runners may not have the @actions/artifact package installed, so install it if needed.
	// by installing in the tool directory, we get caching for free
	toolDir := filepath.Join(template.Render(config.ToolDir), ".node")
	ensureActionsArtifactInstalled(toolDir)
	nodeModulesPath := filepath.Join(toolDir, "node_modules")
	const nodePathEnv = "NODE_PATH"
	if os.Getenv(nodePathEnv) != "" {
		nodeModulesPath += string(os.PathListSeparator) + os.Getenv(nodePathEnv)
	}

	artifactName := opts.ArtifactName
	if artifactName == "" {
		artifactName = filepath.Base(baseDir) + MatrixSuffix
	}

	if opts.Overwrite {
		p := Payload()
		existing := lang.Continue(a.ListArtifactsForWorkflowRun(p.RunID, artifactName))
		if len(existing) > 0 {
			for _, e := range existing {
				log.Error(a.DeleteArtifact(e.ID))
			}
		}
	}

	files := listMatchingFiles(baseDir, &opts)
	if len(files) == 0 {
		return 0, fmt.Errorf("no files to upload dir: %v with opts: %#v", baseDir, opts)
	}

	log.Debug("uploading dir: %s name: %s with files: %v", baseDir, artifactName, files)

	var envs []run.Option
	for k, v := range envFile() {
		envs = append(envs, run.Env(k, v))
	}

	id := node.Run(`
const { DefaultArtifactClient } = require('@actions/artifact')
const artifact = new DefaultArtifactClient()
const archiveName = process.argv[1]
const baseDir = process.argv[2]
const retentionDays = process.argv[3]
const files = process.argv.slice(4)
let opts = {}
if (retentionDays !== "") {
	const intVal = parseInt(retentionDays)
	if (intVal) {
		opts = { retentionDays: intVal }
	}
}
const stdout = process.stdout.write
process.stdout.write = (...args) => process.stderr.write(...args)
Promise.all([artifact.uploadArtifact(archiveName, files, baseDir, opts).then(({ id }) => {
	process.stdout.write = stdout
	console.log(id)
}).catch(err => {
	console.error(err)
	process.exit(1)
})])`,
		run.Env(nodePathEnv, nodeModulesPath),
		run.Options(envs...),
		run.Args("--input-type=commonjs", "--"),
		run.Args(artifactName, baseDir, strconv.Itoa(int(opts.RetentionDays))),
		run.Args(files...),
		run.Quiet())

	log.Debug("uploaded artifact '%s' with id: %s", artifactName, id)

	return strconv.ParseInt(id, 10, 64)
}

func listMatchingFiles(baseDir string, opts *UploadArtifactOption) []string {
	var out []string
	baseDir = lang.Return(filepath.Abs(baseDir))
	for _, f := range opts.Files {
		if !filepath.IsAbs(f) {
			f = filepath.Join(baseDir, f)
		}
		out = append(out, f)
	}
	if len(opts.Files) == 0 && opts.Glob == "" {
		opts.Glob = "**/*" // default to include all files in the baseDir
	}
	if opts.Glob != "" {
		fs := os.DirFS(baseDir)
		globbed := lang.Return(doublestar.Glob(fs, opts.Glob, doublestar.WithFilesOnly(), doublestar.WithNoFollow()))
		for _, f := range globbed {
			if !filepath.IsAbs(f) {
				f = filepath.Join(baseDir, filepath.FromSlash(f))
			}
			out = append(out, f)
		}
	}
	return out
}

func ensureActionsArtifactInstalled(path string) {
	file.EnsureDir(path)
	file.InDir(path, func() {
		if !isActionsArtifactInstalled() {
			if nil != lang.Catch(func() {
				Run("npm install @actions/artifact@latest")
			}) {
				Run("npm install @actions/artifact@" + knownActionsArtifactVersion)
			}
		}
	})
}

func isActionsArtifactInstalled() bool {
	return strings.Contains(Run("npm list @actions/artifact", run.Quiet(), run.NoFail()), "@actions/artifact")
}
