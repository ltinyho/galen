package galen

import (
	"io"
	"text/template"

	"github.com/ltinyho/galen/gdebugserver"
)

// Release is the current galen version.
const Release = "v0.0.1"
// predefined version params
var (
	Version   = "unknown-version"
	BuildTime = "unknown-buildtime"
	GitHash   = "unknown-githash"
	GoVersion = "unknown-goversion"
)

var versionTemplate = `
App: {{.Name}}
  Version:      {{.Version}}
  Build time:   {{.BuildTime}}
  GitHash:      {{.GitHash}}
  Go version:   {{.GoVersion}}
`

func showAppVersion(w io.Writer, name string) error {
	t := template.Must(template.New("version").Parse(versionTemplate))

	err := t.Execute(w, map[string]string{
		"Name":      name,
		"Version":   Version,
		"BuildTime": BuildTime,
		"GitHash":   GitHash,
		"GoVersion": GoVersion,
	})

	return err
}

func init() {
	gdebugserver.SetVersionInfo(map[string]string{
		"Version":   Version,
		"BuildTime": BuildTime,
		"GitHash":   GitHash,
		"GoVersion": GoVersion,
	})
}
