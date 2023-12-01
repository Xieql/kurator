package render

import (
	"bytes"
	"html/template"
	"io/fs"

	"github.com/Masterminds/sprig/v3"
	"sigs.k8s.io/yaml"
)

// renderPipelineTemplate reads, parses, and renders a template file using the provided configuration data.
func renderPipelineTemplate(fsys fs.FS, tplFileName, tplName string, cfg interface{}) ([]byte, error) {
	out, err := fs.ReadFile(fsys, tplFileName)
	if err != nil {
		return nil, err
	}

	t := template.New(tplName)

	tpl, err := t.Funcs(funMap()).Parse(string(out))
	if err != nil {
		return nil, err
	}

	var b bytes.Buffer

	if err := tpl.Execute(&b, cfg); err != nil {
		return nil, err
	}

	return b.Bytes(), nil
}

// funMap returns a map of functions for use in the template.
func funMap() template.FuncMap {
	m := sprig.TxtFuncMap()
	m["toYaml"] = toYaml
	return m
}

// toYaml converts a given value to its YAML representation.
func toYaml(value interface{}) string {
	y, err := yaml.Marshal(value)
	if err != nil {
		return ""
	}

	return string(y)
}
