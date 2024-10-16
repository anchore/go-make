package make

import (
	"bytes"
	"os"
	"path/filepath"
	"reflect"
	"runtime"
	"text/template"
)

var (
	ToolDir  = "{{RootDir}}/.tool"
	Platform = "{{OS}}/{{Arch}}"
	OS       = runtime.GOOS
	Arch     = runtime.GOARCH
)

type ToolContext struct {
	RootDir string
	ToolDir string
}

type Context map[string]any

func (c *Context) Append(key string, value any) {
	(*c)[key] = value
}

var Globals = Context{}

func init() {
	Globals.Append("RootDir", RootDir)
	Globals.Append("RepoRoot", RepoRoot)
	Globals.Append("ToolDir", tplFunc(ToolDir))
	Globals.Append("OS", tplFunc(OS))
	Globals.Append("Arch", tplFunc(Arch))
	Globals.Append("Platform", tplFunc(Platform))
}

func RunTools() {
	defer HandleErrors()
	RunBinny()
	RunGoTask()
}

func RunBinny() {
	Run("binny", "install", "-v")
}

func RunGoTask() {
	defer appendStackOnPanic()
	if findFile(RootDir(), "Taskfile.yaml") == "" {
		return
	}
	if FileExists(ToolPath("task")) {
		NoErr(Exec(ToolPath("task"), ExecArgs(os.Args[1:]...), ExecStd()))
	}
}

func ToolPath(toolName string) string {
	toolPath := toolName
	if runtime.GOOS == "windows" {
		toolPath += ".exe"
	}
	p := filepath.Join(Tpl(ToolDir), toolPath)
	return p
}

func Tpl(template string, args ...map[string]any) string {
	context := map[string]any{}
	for k, v := range Globals {
		if reflect.TypeOf(v).Kind() != reflect.Func {
			context[k] = v
		}
	}
	for _, arg := range args {
		for k, v := range arg {
			context[k] = v
		}
	}
	return render(template, context)
}

func render(tpl string, context map[string]any) string {
	funcs := template.FuncMap{}
	for k, v := range Globals {
		v := v
		val := reflect.ValueOf(v)
		switch val.Type().Kind() {
		case reflect.Func:
			funcs[k] = v
		case reflect.String:
			funcs[k] = func() string { return Tpl(val.String()) }
		default:
			funcs[k] = func() any { return v }
		}
	}
	t := Get(template.New(tpl).Funcs(funcs).Parse(tpl))
	var buf bytes.Buffer
	NoErr(t.Execute(&buf, context))
	return buf.String()
}

func tplFunc(tpl string) func() string {
	return func() string {
		return Tpl(tpl)
	}
}
