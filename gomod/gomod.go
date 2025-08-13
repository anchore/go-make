package gomod

import (
	"os"

	"golang.org/x/mod/modfile"

	"github.com/anchore/go-make/file"
	"github.com/anchore/go-make/git"
	"github.com/anchore/go-make/lang"
)

// Read reads the first go.mod found
func Read() *modfile.File {
	modFile := file.FindParent(file.Cwd(), "go.mod")
	if modFile == "" {
		return nil
	}
	contents := lang.Return(os.ReadFile(modFile))
	return lang.Return(modfile.Parse("go.mod", contents, nil))
}

// GoDepVersion returns the version found for the requested dependency in the first go.mod file found
func GoDepVersion(module string) string {
	f := Read()
	if f.Module != nil && f.Module.Mod.Path == module {
		return git.Revision()
	}
	for _, r := range f.Require {
		if r.Mod.Path == module {
			return r.Mod.Version
		}
	}
	return "UNKNOWN"
}
