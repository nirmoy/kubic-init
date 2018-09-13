package util

import (
	"bytes"
	"encoding/base64"
	"fmt"
	"strings"
	"text/template"
)

// ParseTemplate processes a text/template, doing some replacements
func ParseTemplate(templateStr string, replacements interface{}) (string, error) {

	indent := func(spaces int, v string) string {
		pad := strings.Repeat(" ", spaces)
		return strings.Replace(v, "\n", "\n"+pad, -1)
	}

	replace := func(old, new, src string) string {
		return strings.Replace(src, old, new, -1)
	}

	base64encode := func(v string) string {
		return base64.StdEncoding.EncodeToString([]byte(v))
	}

	base64decode := func(v string) string {
		data, err := base64.StdEncoding.DecodeString(v)
		if err != nil {
			return err.Error()
		}
		return string(data)
	}

	// some custom functions
	funcMap := template.FuncMap{
		"indent":  indent,
		"replace": replace,
		"base64encode": base64encode,
		"base64decode": base64decode,
	}

	var buf bytes.Buffer
	tmpl, err := template.New("template").Funcs(funcMap).Parse(templateStr)
	if err != nil {
		return "", fmt.Errorf("error when parsing template: %v", err)
	}

	err = tmpl.Execute(&buf, replacements)
	if err != nil {
		return "", fmt.Errorf("error when executing template: %v", err)
	}

	contents := buf.String()
	if err != nil {
		return "", fmt.Errorf("error when parsing AutoYaST template: %v", err)
	}

	return contents, nil
}
