package auth

import (
	"io/fs"
	"os"
	"path"
	"path/filepath"
	"reflect"
	"strings"
	"testing"
	"testing/fstest"
)

func TestDiscoverCombinesDefaultAndExtensionPackages(t *testing.T) {
	t.Parallel()
	defaults := packageFS("apiKey")
	extensions := packageFS("companyAuth")
	extensions[packagePath("companyAuth", "references.yaml")] = &fstest.MapFile{
		Data: []byte("secretFiles:\n  directoryFile: /directory/secretRef\n"),
	}
	catalog, err := Discover(
		Source{Name: "defaults", FS: defaults, Required: true},
		Source{Name: "extensions", FS: extensions},
	)
	if err != nil {
		t.Fatal(err)
	}
	if len(catalog) != 2 {
		t.Fatalf("got %d packages, want 2", len(catalog))
	}
	got := catalog["companyAuth"]
	if got.AuthType != "companyAuth" || got.Source != "extensions" || got.ChartName != "auth-example" || got.Version != "1.2.3" {
		t.Fatalf("unexpected extension metadata: %+v", got)
	}
	if !reflect.DeepEqual(got.SecretFiles, map[string]string{"directoryFile": "/directory/secretRef"}) {
		t.Fatalf("unexpected reference declaration: %v", got.SecretFiles)
	}
	if _, found := catalog["missingAuth"]; found {
		t.Fatal("catalog contains a type that was not installed")
	}
	repeated, err := Discover(
		Source{Name: "defaults", FS: defaults, Required: true},
		Source{Name: "extensions", FS: extensions},
	)
	if err != nil || !reflect.DeepEqual(catalog, repeated) {
		t.Fatalf("unchanged sources produced different catalogs: %v", err)
	}
}

func TestDiscoverSnapshotsAllChartFiles(t *testing.T) {
	t.Parallel()
	source := packageFS("apiKey")
	source[packagePath("apiKey", "values.yaml")] = &fstest.MapFile{Data: []byte("header: X-API-Key\n")}
	source[packagePath("apiKey", "templates/_helpers.tpl")] = &fstest.MapFile{Data: []byte("{{- define \"name\" -}}auth{{- end -}}")}
	source[packagePath("apiKey", "assets/notes.txt")] = &fstest.MapFile{Data: []byte("Packaged chart data.\n")}
	catalog, err := Discover(Source{Name: "defaults", FS: source, Required: true})
	if err != nil {
		t.Fatal(err)
	}
	for _, name := range []string{"values.yaml", "templates/_helpers.tpl", "assets/notes.txt"} {
		want := string(source[packagePath("apiKey", name)].Data)
		source[packagePath("apiKey", name)].Data[0] = '!'
		if got := string(catalog["apiKey"].Files[name]); got != want {
			t.Fatalf("source mutation changed snapshotted %s: %q", name, got)
		}
	}
	delete(source, packagePath("apiKey", "Chart.yaml"))
	if len(catalog["apiKey"].Files["Chart.yaml"]) == 0 {
		t.Fatal("source removal changed the package snapshot")
	}
}

func TestDiscoverWithoutSecretFiles(t *testing.T) {
	t.Parallel()
	source := packageFS("externalAuth")
	delete(source, packagePath("externalAuth", "references.yaml"))
	catalog, err := Discover(Source{Name: "defaults", FS: source, Required: true})
	if err != nil {
		t.Fatal(err)
	}
	if len(catalog["externalAuth"].SecretFiles) != 0 {
		t.Fatalf("unexpected Secret dependency: %v", catalog["externalAuth"].SecretFiles)
	}
}

func TestDiscoverDoesNotRequireOrValidateAdmissionArtifacts(t *testing.T) {
	t.Parallel()
	for name, artifact := range map[string]*fstest.MapFile{
		"absent":    nil,
		"unrelated": {Data: []byte("unrelated: [not valid YAML")},
	} {
		t.Run(name, func(t *testing.T) {
			source := packageFS("customAuth")
			filename := packagePath("customAuth", "admission/validation.yaml")
			delete(source, filename)
			if artifact != nil {
				source[filename] = artifact
			}
			catalog, err := Discover(Source{Name: "extensions", FS: source, Required: true})
			if err != nil {
				t.Fatalf("admission artifacts must not control discovery: %v", err)
			}
			if catalog["customAuth"].AuthType != "customAuth" {
				t.Fatal("package was not discovered")
			}
		})
	}
}

func TestDiscoverSourceRequirements(t *testing.T) {
	t.Parallel()
	tests := []struct {
		name     string
		source   Source
		wantFail bool
	}{
		{name: "missing optional subtree", source: Source{Name: "extensions", FS: fstest.MapFS{}}},
		{name: "missing required subtree", source: Source{Name: "defaults", FS: fstest.MapFS{}, Required: true}, wantFail: true},
		{name: "empty optional subtree", source: Source{Name: "extensions", FS: fstest.MapFS{PackageDirectory: {Mode: fs.ModeDir}}}},
		{name: "empty required subtree", source: Source{Name: "defaults", FS: fstest.MapFS{PackageDirectory: {Mode: fs.ModeDir}}, Required: true}, wantFail: true},
		{name: "nil filesystem", source: Source{Name: "defaults"}, wantFail: true},
		{name: "unnamed source", source: Source{FS: packageFS("apiKey")}, wantFail: true},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			_, err := Discover(tt.source)
			if (err != nil) != tt.wantFail {
				t.Fatalf("Discover() error = %v, want failure %v", err, tt.wantFail)
			}
		})
	}
}

func TestDiscoverRejectsDuplicateTypesAcrossSources(t *testing.T) {
	t.Parallel()
	_, err := Discover(
		Source{Name: "defaults", FS: packageFS("apiKey")},
		Source{Name: "extensions", FS: packageFS("apiKey")},
	)
	if err == nil {
		t.Fatal("duplicate package unexpectedly replaced the default")
	}
	for _, detail := range []string{"apiKey", "defaults", "extensions"} {
		if !strings.Contains(err.Error(), detail) {
			t.Fatalf("error %q does not identify %q", err, detail)
		}
	}
}

func TestDiscoverRejectsIncompletePackages(t *testing.T) {
	t.Parallel()
	for _, filename := range []string{"Chart.yaml", "values.schema.json", "README.md", "templates/contribution.yaml"} {
		t.Run(filename, func(t *testing.T) {
			t.Parallel()
			source := packageFS("apiKey")
			delete(source, packagePath("apiKey", filename))
			_, err := Discover(Source{Name: "defaults", FS: source})
			if err == nil || !strings.Contains(err.Error(), filename) || !strings.Contains(err.Error(), "apiKey") || !strings.Contains(err.Error(), "defaults") {
				t.Fatalf("missing artifact did not produce a contextual error: %v", err)
			}
		})
	}
}

func TestDiscoverRejectsMalformedPackages(t *testing.T) {
	t.Parallel()
	tests := []struct {
		name string
		file string
		data string
	}{
		{name: "malformed chart", file: "Chart.yaml", data: "name: ["},
		{name: "multiple chart documents", file: "Chart.yaml", data: "apiVersion: v2\nname: auth\nversion: 1.2.3\n---\nname: another\n"},
		{name: "missing chart name", file: "Chart.yaml", data: "apiVersion: v2\nversion: 1.2.3\n"},
		{name: "missing chart version", file: "Chart.yaml", data: "apiVersion: v2\nname: auth\n"},
		{name: "relative chart path", file: "Chart.yaml", data: "apiVersion: v2\nname: ../auth\nversion: 1.2.3\n"},
		{name: "nested chart path", file: "Chart.yaml", data: "apiVersion: v2\nname: nested/auth\nversion: 1.2.3\n"},
		{name: "dot chart name", file: "Chart.yaml", data: "apiVersion: v2\nname: .\nversion: 1.2.3\n"},
		{name: "dot dot chart name", file: "Chart.yaml", data: "apiVersion: v2\nname: ..\nversion: 1.2.3\n"},
		{name: "invalid chart version", file: "Chart.yaml", data: "apiVersion: v2\nname: auth\nversion: nonsense\n"},
		{name: "old chart API", file: "Chart.yaml", data: "apiVersion: v1\nname: auth\nversion: 1.2.3\n"},
		{name: "library chart", file: "Chart.yaml", data: "apiVersion: v2\nname: auth\nversion: 1.2.3\ntype: library\n"},
		{name: "invalid schema JSON", file: "values.schema.json", data: "{"},
		{name: "non-object schema", file: "values.schema.json", data: "true"},
		{name: "null schema", file: "values.schema.json", data: "null"},
		{name: "empty contribution", file: "templates/contribution.yaml", data: "\n"},
		{name: "unknown reference declaration", file: "references.yaml", data: "secretFilez:\n  keys: /keySource/secretRef\n"},
		{name: "multiple reference documents", file: "references.yaml", data: "secretFiles: {}\n---\nunknown: ignored\n"},
		{name: "duplicate reference slot", file: "references.yaml", data: "secretFiles:\n  keys: /one\n  keys: /two\n"},
		{name: "empty reference slot", file: "references.yaml", data: "secretFiles:\n  '': /keySource/secretRef\n"},
		{name: "non-pointer reference", file: "references.yaml", data: "secretFiles:\n  keys: keySource.secretRef\n"},
		{name: "invalid pointer escape", file: "references.yaml", data: "secretFiles:\n  keys: /key~2source/secretRef\n"},
		{name: "trailing pointer escape", file: "references.yaml", data: "secretFiles:\n  keys: /keySource~\n"},
		{name: "non-string reference", file: "references.yaml", data: "secretFiles:\n  keys: 42\n"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			source := packageFS("apiKey")
			source[packagePath("apiKey", tt.file)] = &fstest.MapFile{Data: []byte(tt.data)}
			_, err := Discover(Source{Name: "defaults", FS: source})
			if err == nil || !strings.Contains(err.Error(), tt.file) {
				t.Fatalf("malformed artifact did not produce a file-specific error: %v", err)
			}
		})
	}
}

func TestDiscoverAcceptsPointerEscapesAndEmptyChartType(t *testing.T) {
	t.Parallel()
	source := packageFS("customAuth")
	source[packagePath("customAuth", "Chart.yaml")] = &fstest.MapFile{Data: []byte("apiVersion: v2\nname: auth-example\nversion: 1.2.3\n")}
	source[packagePath("customAuth", "references.yaml")] = &fstest.MapFile{Data: []byte("secretFiles:\n  keys: /key~1source/~0secretRef\n")}
	if _, err := Discover(Source{Name: "extensions", FS: source}); err != nil {
		t.Fatal(err)
	}
}

func TestDiscoverProjectedVolumeLayout(t *testing.T) {
	t.Parallel()
	directory := t.TempDir()
	data := filepath.Join(directory, "..2026_11_27")
	packageDirectory := filepath.Join(data, filepath.FromSlash(PackageDirectory), "apiKey")
	for name, file := range packageFS("apiKey") {
		filename := filepath.Join(data, filepath.FromSlash(name))
		if err := os.MkdirAll(filepath.Dir(filename), 0o750); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(filename, file.Data, 0o600); err != nil {
			t.Fatal(err)
		}
	}
	// AtomicWriter uses hidden data directories and symlinks for projected keys.
	if err := os.Symlink("..2026_11_27", filepath.Join(directory, "..data")); err != nil {
		t.Fatal(err)
	}
	if err := os.Symlink("..data/aigateway", filepath.Join(directory, "aigateway")); err != nil {
		t.Fatal(err)
	}
	if err := os.Rename(filepath.Join(packageDirectory, "templates"), filepath.Join(packageDirectory, "..templates")); err != nil {
		t.Fatal(err)
	}
	if err := os.Symlink("..templates", filepath.Join(packageDirectory, "templates")); err != nil {
		t.Fatal(err)
	}
	root, err := os.OpenRoot(directory)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		if err := root.Close(); err != nil {
			t.Errorf("close package root: %v", err)
		}
	})
	catalog, err := Discover(Source{Name: "extensions", FS: root.FS(), Required: true})
	if err != nil {
		t.Fatal(err)
	}
	if len(catalog) != 1 || string(catalog["apiKey"].Files["templates/contribution.yaml"]) != "plugins: []\n" {
		t.Fatalf("projected package was not snapshotted correctly: %+v", catalog)
	}
}

func packagePath(authType, filename string) string {
	return path.Join(PackageDirectory, authType, filename)
}

func packageFS(authType string) fstest.MapFS {
	files := map[string]string{
		"Chart.yaml":                  "apiVersion: v2\nname: auth-example\nversion: 1.2.3\ntype: application\n",
		"values.schema.json":          `{"type":"object"}`,
		"README.md":                   "Authentication package.\n",
		"templates/contribution.yaml": "plugins: []\n",
		"references.yaml":             "secretFiles:\n  keys: /keySource/secretRef\n",
	}
	source := fstest.MapFS{}
	for filename, data := range files {
		source[packagePath(authType, filename)] = &fstest.MapFile{Data: []byte(data)}
	}
	return source
}
