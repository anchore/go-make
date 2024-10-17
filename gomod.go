package make

import (
	"os"

	"golang.org/x/mod/modfile"
)

// ReadGoMod reads the first go.mod found
func ReadGoMod() *modfile.File {
	modFile := FindFile("go.mod")
	contents := Get(os.ReadFile(modFile))
	return Get(modfile.Parse("go.mod", contents, nil))
}

// GoDepVersion returns the version found for the requested dependency in the first go.mod file found
func GoDepVersion(module string) string {
	f := ReadGoMod()
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
