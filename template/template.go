package template

import (
	"bytes"
	"reflect"
	"text/template"

	"github.com/anchore/go-make/config"
)

// Globals is a map of template variables available in all Render calls.
// Built-in variables include ToolDir, RootDir, OS, and Arch. Packages can
// add their own globals (e.g., git package adds GitRoot).
//
// To add custom variables:
//
//	template.Globals["Version"] = func() string { return "1.0.0" }
//	template.Globals["BuildTime"] = time.Now().Format(time.RFC3339)
var Globals = map[string]any{}

func init() {
	Globals["ToolDir"] = renderFunc(&config.ToolDir)
	Globals["RootDir"] = renderFunc(&config.RootDir)
	Globals["OS"] = renderFunc(&config.OS)
	Globals["Arch"] = renderFunc(&config.Arch)
}

// Render processes a Go template string, substituting variables from Globals
// and any additional context maps provided. Variables can be values or functions
// (which are called during rendering).
//
// Example:
//
//	Render("{{RootDir}}/.tool/{{OS}}_{{Arch}}")
//	Render("Hello {{.Name}}", map[string]any{"Name": "World"})
func Render(template string, args ...map[string]any) string {
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
		val := reflect.ValueOf(v)
		switch val.Type().Kind() {
		case reflect.Func:
			funcs[k] = v
		case reflect.String:
			funcs[k] = func() string { return Render(val.String()) }
		default:
			funcs[k] = func() any { return v }
		}
	}
	t, err := template.New(tpl).Funcs(funcs).Parse(tpl)
	if err != nil {
		panic(err)
	}
	var buf bytes.Buffer
	err = t.Execute(&buf, context)
	if err != nil {
		panic(err)
	}
	return buf.String()
}

func renderFunc(template *string) func() string {
	return func() string {
		return Render(*template)
	}
}
