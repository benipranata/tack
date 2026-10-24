// Package render turns a bound output into the Go source tack writes.
package render

import (
	"bytes"
	"embed"
	"fmt"
	"go/format"
	"path"
	"text/template"

	"github.com/benipranata/tack/internal/config"
	"github.com/benipranata/tack/internal/load"
)

//go:embed templates/*.tmpl
var templateFS embed.FS

var templates = template.Must(template.ParseFS(templateFS, "templates/*.tmpl"))

// File is a file to write, or to delete when Remove is set. Path is relative to the module root.
type File struct {
	Path    string
	Content []byte
	Remove  bool
}

// job is one file to generate from a template.
type job struct {
	path, template string
	data           any
}

// Render returns the constructor file of out and its test double, which is removed when out has none.
func Render(out load.Output) ([]File, error) {
	jobs := []job{{path.Join(out.Package.Dir, out.Filename), "constructor.go.tmpl", newConstructor(out)}}

	doublePath := path.Join(out.Package.Dir, config.DoubleFilename(out.Filename))
	if out.Test {
		jobs = append(jobs, job{doublePath, "double.go.tmpl", newDouble(out)})
	}

	var files []File
	for _, j := range jobs {
		src, err := execute(j.template, j.data)
		if err != nil {
			return nil, fmt.Errorf("%s: %w", j.path, err)
		}
		files = append(files, File{Path: j.path, Content: src})
	}
	if !out.Test {
		files = append(files, File{Path: doublePath, Remove: true})
	}
	return files, nil
}

// execute renders the named template and formats the result.
func execute(name string, data any) ([]byte, error) {
	var buf bytes.Buffer
	if err := templates.ExecuteTemplate(&buf, name, data); err != nil {
		return nil, err
	}
	src, err := format.Source(buf.Bytes())
	if err != nil {
		return nil, fmt.Errorf("format generated code: %w\n%s", err, buf.Bytes())
	}
	return src, nil
}
