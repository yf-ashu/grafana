package codegen

import (
	"bytes"
	gerrors "errors"
	"fmt"
	"io"
	"io/fs"
	"os"
	"path/filepath"
	"strings"
	"testing/fstest"
	"text/template"

	"cuelang.org/go/cue"
	"cuelang.org/go/cue/ast"
	"cuelang.org/go/cue/errors"
	cload "cuelang.org/go/cue/load"
	"cuelang.org/go/cue/parser"
	"github.com/grafana/cuetsy"
	"github.com/grafana/grafana/pkg/schema/load"
)

// The only import statement we currently allow in any models.cue file
const allowedImport = "github.com/grafana/grafana/packages/grafana-schema/src/schema"

var importMap = map[string]string{
	allowedImport: "@grafana/schema",
}

// Hard-coded list of paths to skip. Remove a particular file as we're ready
// to rely on the TypeScript auto-generated by cuetsy for that particular file.
var skipPaths = []string{
	"public/app/plugins/panel/barchart/models.cue",
	"public/app/plugins/panel/canvas/models.cue",
	"public/app/plugins/panel/histogram/models.cue",
	"public/app/plugins/panel/heatmap/models.cue",
	"public/app/plugins/panel/heatmap-old/models.cue",
	"public/app/plugins/panel/candlestick/models.cue",
	"public/app/plugins/panel/state-timeline/models.cue",
	"public/app/plugins/panel/status-history/models.cue",
	"public/app/plugins/panel/table/models.cue",
	"public/app/plugins/panel/timeseries/models.cue",
}

const prefix = "/"

var paths = load.GetDefaultLoadPaths()

// CuetsifyPlugins runs cuetsy against plugins' models.cue files.
func CuetsifyPlugins(ctx *cue.Context, root string) (WriteDiffer, error) {
	// TODO this whole func has a lot of old, crufty behavior from the scuemata era; needs TLC
	var fspaths load.BaseLoadPaths
	var err error

	fspaths.BaseCueFS, err = populateMapFSFromRoot(paths.BaseCueFS, root, "")
	if err != nil {
		return nil, err
	}
	fspaths.DistPluginCueFS, err = populateMapFSFromRoot(paths.DistPluginCueFS, root, "")
	if err != nil {
		return nil, err
	}
	overlay, err := defaultOverlay(fspaths)
	if err != nil {
		return nil, err
	}

	// Prep the cue load config
	clcfg := &cload.Config{
		Overlay: overlay,
		// FIXME these module paths won't work for things not under our cue.mod - AKA third-party plugins
		ModuleRoot: prefix,
		Module:     "github.com/grafana/grafana",
	}

	// FIXME hardcoding paths to exclude is not the way to handle this
	excl := map[string]bool{
		"cue.mod":      true,
		"cue/scuemata": true,
		"packages/grafana-schema/src/scuemata/dashboard":      true,
		"packages/grafana-schema/src/scuemata/dashboard/dist": true,
	}

	exclude := func(path string) bool {
		dir := filepath.Dir(path)
		if excl[dir] {
			return true
		}
		for _, p := range skipPaths {
			if path == p {
				return true
			}
		}

		return false
	}

	outfiles := NewWriteDiffer()

	cuetsify := func(in fs.FS) error {
		seen := make(map[string]bool)
		return fs.WalkDir(in, ".", func(path string, d fs.DirEntry, err error) error {
			if err != nil {
				return err
			}
			dir := filepath.Dir(path)

			if d.IsDir() || filepath.Ext(d.Name()) != ".cue" || seen[dir] || exclude(path) {
				return nil
			}
			seen[dir] = true
			clcfg.Dir = filepath.Join(root, dir)
			// FIXME Horrible hack to figure out the identifier used for
			// imported packages - intercept the parser called by the loader to
			// look at the ast.Files on their way in to building.
			// Much better if we could work backwards from the cue.Value,
			// maybe even directly in cuetsy itself, and figure out when a
			// referenced object is "out of bounds".
			// var imports sync.Map
			var imports []*ast.ImportSpec
			clcfg.ParseFile = func(name string, src interface{}) (*ast.File, error) {
				f, err := parser.ParseFile(name, src, parser.ParseComments)
				if err != nil {
					return nil, err
				}
				imports = append(imports, f.Imports...)
				return f, nil
			}
			if strings.Contains(path, "public/app/plugins") {
				clcfg.Package = "grafanaschema"
			} else {
				clcfg.Package = ""
			}

			// FIXME loading in this way causes all files in a dir to be loaded
			// as a single cue.Instance or cue.Value, which makes it quite
			// difficult to map them _back_ onto the original file and generate
			// discrete .gen.ts files for each .cue input.  However, going one
			// .cue file at a time and passing it as the first arg to
			// load.Instances() means that the other files are ignored
			// completely, causing references between these files to be
			// unresolved, and thus encounter a different kind of error.
			insts := cload.Instances(nil, clcfg)
			if len(insts) > 1 {
				panic("extra instances")
			}
			bi := insts[0]

			v := ctx.BuildInstance(bi)
			if v.Err() != nil {
				return v.Err()
			}

			var b []byte
			f := &tsFile{}
			seen := make(map[string]bool)
			// FIXME explicitly mapping path patterns to conversion patterns
			// is exactly what we want to avoid
			switch {
			// panel plugin models.cue files
			case strings.Contains(path, "public/app/plugins"):
				for _, im := range imports {
					ip := strings.Trim(im.Path.Value, "\"")
					if ip != allowedImport {
						// TODO make a specific error type for this
						return errors.Newf(im.Pos(), "import %q not allowed, panel plugins may only import from %q", ip, allowedImport)
					}
					// TODO this approach will silently swallow the unfixable
					// error case where multiple files in the same dir import
					// the same package to a different ident
					if !seen[ip] {
						seen[ip] = true
						f.Imports = append(f.Imports, convertImport(im))
					}
				}

				// Extract the latest schema and its version number. (All of this goes away with Thema, whew)
				f.V = &tsModver{}
				lins := v.LookupPath(cue.ParsePath("Panel.lineages"))
				f.V.Lin, _ = lins.Len().Int64()
				f.V.Lin = f.V.Lin - 1
				schs := lins.LookupPath(cue.MakePath(cue.Index(int(f.V.Lin))))
				f.V.Sch, _ = schs.Len().Int64()
				f.V.Sch = f.V.Sch - 1
				latest := schs.LookupPath(cue.MakePath(cue.Index(int(f.V.Sch))))

				b, err = cuetsy.Generate(latest, cuetsy.Config{})
			default:
				b, err = cuetsy.Generate(v, cuetsy.Config{})
			}

			if err != nil {
				return err
			}
			f.Body = string(b)

			var buf bytes.Buffer
			err = tsTemplate.Execute(&buf, f)
			outfiles[filepath.Join(root, strings.Replace(path, ".cue", ".gen.ts", -1))] = buf.Bytes()
			return err
		})
	}

	err = cuetsify(fspaths.BaseCueFS)
	if err != nil {
		return nil, gerrors.New(errors.Details(err, nil))
	}
	err = cuetsify(fspaths.DistPluginCueFS)
	if err != nil {
		return nil, gerrors.New(errors.Details(err, nil))
	}

	return outfiles, nil
}

func convertImport(im *ast.ImportSpec) *tsImport {
	tsim := &tsImport{
		Pkg: importMap[allowedImport],
	}
	if im.Name != nil && im.Name.String() != "" {
		tsim.Ident = im.Name.String()
	} else {
		sl := strings.Split(im.Path.Value, "/")
		final := sl[len(sl)-1]
		if idx := strings.Index(final, ":"); idx != -1 {
			tsim.Pkg = final[idx:]
		} else {
			tsim.Pkg = final
		}
	}
	return tsim
}

func defaultOverlay(p load.BaseLoadPaths) (map[string]cload.Source, error) {
	overlay := make(map[string]cload.Source)

	if err := toOverlay(prefix, p.BaseCueFS, overlay); err != nil {
		return nil, err
	}

	if err := toOverlay(prefix, p.DistPluginCueFS, overlay); err != nil {
		return nil, err
	}

	return overlay, nil
}

func toOverlay(prefix string, vfs fs.FS, overlay map[string]cload.Source) error {
	if !filepath.IsAbs(prefix) {
		return fmt.Errorf("must provide absolute path prefix when generating cue overlay, got %q", prefix)
	}
	err := fs.WalkDir(vfs, ".", func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}

		if d.IsDir() {
			return nil
		}

		f, err := vfs.Open(path)
		if err != nil {
			return err
		}
		defer func(f fs.File) {
			err := f.Close()
			if err != nil {
				return
			}
		}(f)

		b, err := io.ReadAll(f)
		if err != nil {
			return err
		}

		overlay[filepath.Join(prefix, path)] = cload.FromBytes(b)
		return nil
	})

	if err != nil {
		return err
	}

	return nil
}

// Helper function that populates an fs.FS by walking over a virtual filesystem,
// and reading files from disk corresponding to each file encountered.
func populateMapFSFromRoot(in fs.FS, root, join string) (fs.FS, error) {
	out := make(fstest.MapFS)
	err := fs.WalkDir(in, ".", func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}

		if d.IsDir() {
			return nil
		}
		// Ignore gosec warning G304. The input set here is necessarily
		// constrained to files specified in embed.go
		// nolint:gosec
		b, err := os.Open(filepath.Join(root, join, path))
		if err != nil {
			return err
		}
		byt, err := io.ReadAll(b)
		if err != nil {
			return err
		}

		out[path] = &fstest.MapFile{Data: byt}
		return nil
	})
	return out, err
}

type tsFile struct {
	V       *tsModver
	Imports []*tsImport
	Body    string
}

type tsModver struct {
	Lin, Sch int64
}

type tsImport struct {
	Ident string
	Pkg   string
}

var tsTemplate = template.Must(template.New("cuetsygen").Parse(`//~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~
// This file is autogenerated. DO NOT EDIT.
//
// To regenerate, run "make gen-cue" from the repository root.
//~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~
{{range .Imports}}
import * as {{.Ident}} from '{{.Pkg}}';{{end}}
{{if .V}}
export const modelVersion = Object.freeze([{{ .V.Lin }}, {{ .V.Sch }}]);
{{end}}
{{.Body}}`))
