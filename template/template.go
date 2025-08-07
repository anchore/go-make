package template

import (
	"bytes"
	"reflect"
	"text/template"

	"github.com/anchore/go-make/config"
)

var Globals = map[string]any{}

func init() {
	Globals["ToolDir"] = renderFunc(&config.ToolDir)
	Globals["RootDir"] = renderFunc(&config.RootDir)
	Globals["OS"] = renderFunc(&config.OS)
	Globals["Arch"] = renderFunc(&config.Arch)
	Globals["Platform"] = renderFunc(&config.Platform)
}

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
