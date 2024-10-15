package make

import (
	"os"

	"golang.org/x/mod/modfile"
)

func GoDepVersion(module string) string {
	modFile := FindFile("go.mod")
	contents := Get(os.ReadFile(modFile))
	f := Get(modfile.Parse("go.mod", contents, nil))
	if f.Module != nil && f.Module.Mod.Path == module {
		return GitRevision()
	}
	for _, r := range f.Require {
		if r.Mod.Path == module {
			return r.Mod.Version
		}
	}
	return "UNKNOWN"
}
