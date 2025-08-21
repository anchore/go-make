package github

import (
	"fmt"
	"path"
	"path/filepath"
	"strconv"
	"strings"

	"github.com/anchore/go-make/file"
	"github.com/anchore/go-make/lang"
	"github.com/anchore/go-make/log"
	"github.com/anchore/go-make/node"
	"github.com/anchore/go-make/run"
	"github.com/anchore/go-make/script"
)

const (
	knownActionsArtifactVersion = "2.3.2"
)

type UploadArtifactOption struct {
	ArtifactName  string
	RetentionDays uint
	Glob          string
	Files         []string
}

// UploadArtifactDir will compress all files in the basedir into an artifact
// attached to the currently running workflow. Optionally limit the files by
// including ArtifactFiles or ArtifactGlob, which can be relative to the baseDir.
// To specify a custom name, use ArtifactName
func (a Api) UploadArtifactDir(baseDir string, opts UploadArtifactOption) (int64, error) {
	artifactName := opts.ArtifactName
	if artifactName == "" {
		artifactName = filepath.Base(baseDir)
	}

	files := renderUploadFiles(baseDir, &opts)
	if len(files) == 0 {
		return 0, fmt.Errorf("no files to upload dir: %v with opts: %#v", baseDir, opts)
	}

	// Github actions runners may not have the @actions/artifact package installed, so install it if needed
	ensureActionsArtifactInstalled()

	log.Debug("uploading dir: %s name: %s with files: %v", baseDir, artifactName, files)

	// `npm install -g @actions/artifact` is available, but import fails at: $(npm -g root)/@actions/artifact
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
Promise.all([artifact.uploadArtifact(archiveName, files, baseDir, opts).then(({ id }) => {
	console.log(id)
}).catch(err => {
	console.error(err)
	process.exit(1)
})])`,
		run.Args(artifactName, baseDir, strconv.Itoa(int(opts.RetentionDays))),
		run.Args(files...),
		run.Env("ACTIONS_RUNTIME_TOKEN", a.Token))

	return strconv.ParseInt(id, 10, 64)
}

func renderUploadFiles(baseDir string, opts *UploadArtifactOption) []string {
	var out []string
	baseDir = lang.Return(filepath.Abs(baseDir))
	for _, f := range opts.Files {
		if !filepath.IsAbs(f) {
			f = path.Join(baseDir, f)
		}
		out = append(out, f)
	}
	if len(opts.Files) == 0 && opts.Glob == "" {
		opts.Glob = "**/*" // default to include all files in the baseDir
	}
	if opts.Glob != "" {
		glob := opts.Glob
		if !filepath.IsAbs(glob) {
			glob = filepath.Join(baseDir, glob)
		}
		for _, f := range file.FindAll(glob) {
			if !filepath.IsAbs(f) {
				f = filepath.Join(baseDir, f)
			}
			out = append(out, f)
		}
	}
	return out
}

func ensureActionsArtifactInstalled() {
	if !isActionsArtifactInstalled() {
		if nil != lang.Catch(func() {
			script.Run("npm install @actions/artifact@latest")
		}) {
			script.Run("npm install @actions/artifact@" + knownActionsArtifactVersion)
		}
	}
}

func isActionsArtifactInstalled() bool {
	return strings.Contains(script.Run("npm list @actions/artifact", run.Quiet(), run.NoFail()), "@actions/artifact")
}
