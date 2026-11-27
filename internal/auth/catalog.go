// Package auth discovers platform-managed authentication rendering packages.
package auth

import (
	"bufio"
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"io/fs"
	"path"
	"strings"

	"k8s.io/apimachinery/pkg/util/version"
	yamlutil "k8s.io/apimachinery/pkg/util/yaml"
	"sigs.k8s.io/yaml"
)

// PackageDirectory is the auth package subtree within every source filesystem.
const PackageDirectory = "aigateway/governance/auth"

// Source is a named package root. Its filesystem must confine reads to the
// approved root, including symlink resolution; os.Root.FS supplies that boundary.
type Source struct {
	Name string
	FS   fs.FS
	// Required requires the auth subtree and at least one package to exist.
	Required bool
}

// Package is a startup snapshot, independent of subsequent source-file updates.
// Files includes chart helpers and defaults for the eventual Helm renderer.
// The caller owns the returned maps and must not mutate them while in use.
type Package struct {
	AuthType    string
	Source      string
	ChartName   string
	Version     string
	Files       map[string][]byte
	SecretFiles map[string]string
}

// Catalog indexes installed packages by auth type, without granting Tenant access.
type Catalog map[string]Package

// Discover merges named sources, rejecting incomplete or conflicting packages.
// It snapshots package files but performs no rendering or Kubernetes operations.
func Discover(sources ...Source) (Catalog, error) {
	catalog := make(Catalog)
	for _, source := range sources {
		if strings.TrimSpace(source.Name) == "" || source.FS == nil {
			return nil, fmt.Errorf("auth source %q requires a name and filesystem", source.Name)
		}
		entries, err := fs.ReadDir(source.FS, PackageDirectory)
		if errors.Is(err, fs.ErrNotExist) && !source.Required {
			continue
		}
		if err != nil {
			return nil, fmt.Errorf("auth source %q: %w", source.Name, err)
		}
		found := 0
		for _, entry := range entries {
			if strings.HasPrefix(entry.Name(), ".") {
				continue
			}
			directory := path.Join(PackageDirectory, entry.Name())
			info, err := fs.Stat(source.FS, directory)
			if err != nil {
				return nil, fmt.Errorf("auth source %q package %q: %w", source.Name, entry.Name(), err)
			}
			if !info.IsDir() {
				continue
			}
			if previous, exists := catalog[entry.Name()]; exists {
				return nil, fmt.Errorf("duplicate auth type %q in sources %q and %q", entry.Name(), previous.Source, source.Name)
			}
			packageFS, err := fs.Sub(source.FS, directory)
			if err != nil {
				return nil, fmt.Errorf("auth source %q package %q: %w", source.Name, entry.Name(), err)
			}
			pkg, err := loadPackage(packageFS)
			if err != nil {
				return nil, fmt.Errorf("auth source %q package %q: %w", source.Name, entry.Name(), err)
			}
			pkg.AuthType, pkg.Source = entry.Name(), source.Name
			catalog[pkg.AuthType] = pkg
			found++
		}
		if source.Required && found == 0 {
			return nil, fmt.Errorf("auth source %q contains no packages", source.Name)
		}
	}
	return catalog, nil
}

func loadPackage(source fs.FS) (Package, error) {
	files := make(map[string][]byte)
	if err := snapshotFiles(source, ".", files); err != nil {
		return Package{}, err
	}
	for _, filename := range []string{"Chart.yaml", "values.schema.json", "README.md", "templates/contribution.yaml", "admission/validation.yaml"} {
		if len(bytes.TrimSpace(files[filename])) == 0 {
			return Package{}, fmt.Errorf("%s is required and must not be empty", filename)
		}
	}
	var chart struct {
		APIVersion string `json:"apiVersion"`
		Name       string `json:"name"`
		Version    string `json:"version"`
		Type       string `json:"type"`
	}
	if err := decodeObject(files["Chart.yaml"], &chart, false); err != nil {
		return Package{}, fmt.Errorf("chart metadata in Chart.yaml: %w", err)
	}
	if chart.APIVersion != "v2" || strings.TrimSpace(chart.Name) == "" || (chart.Type != "" && chart.Type != "application") {
		return Package{}, errors.New("chart metadata in Chart.yaml requires apiVersion v2, a name and application chart type")
	}
	if path.Base(chart.Name) != chart.Name || chart.Name == "." || chart.Name == ".." || strings.Contains(chart.Name, `\`) {
		return Package{}, errors.New("chart name in Chart.yaml must be a basename, not a path")
	}
	if _, err := version.ParseSemantic(chart.Version); err != nil {
		return Package{}, fmt.Errorf("chart version in Chart.yaml: %w", err)
	}
	var schema map[string]json.RawMessage
	if err := json.Unmarshal(files["values.schema.json"], &schema); err != nil || schema == nil {
		return Package{}, errors.New("values.schema.json must contain a JSON object")
	}
	var references struct {
		SecretFiles map[string]string `json:"secretFiles"`
	}
	if data, exists := files["references.yaml"]; exists {
		if err := decodeObject(data, &references, true); err != nil {
			return Package{}, fmt.Errorf("references.yaml: %w", err)
		}
		for slot, pointer := range references.SecretFiles {
			if strings.TrimSpace(slot) == "" || !validPointer(pointer) {
				return Package{}, fmt.Errorf("references.yaml: slot %q requires a name and an RFC6901 pointer starting with /", slot)
			}
		}
	}
	if err := validateAdmission(files["admission/validation.yaml"]); err != nil {
		return Package{}, fmt.Errorf("admission/validation.yaml: %w", err)
	}
	return Package{ChartName: chart.Name, Version: chart.Version, Files: files, SecretFiles: references.SecretFiles}, nil
}

func snapshotFiles(source fs.FS, directory string, files map[string][]byte) error {
	entries, err := fs.ReadDir(source, directory)
	if err != nil {
		return err
	}
	for _, entry := range entries {
		// Kubernetes projected volumes expose hidden backing directories beside
		// symlinks to their current data. Read the visible paths only.
		if strings.HasPrefix(entry.Name(), "..") {
			continue
		}
		filename := path.Join(directory, entry.Name())
		info, err := fs.Stat(source, filename)
		if err != nil {
			return fmt.Errorf("%s: %w", filename, err)
		}
		if info.IsDir() {
			if err := snapshotFiles(source, filename, files); err != nil {
				return err
			}
			continue
		}
		if !info.Mode().IsRegular() {
			return fmt.Errorf("%s is not a regular file", filename)
		}
		data, err := fs.ReadFile(source, filename)
		if err != nil {
			return fmt.Errorf("%s: %w", filename, err)
		}
		files[filename] = bytes.Clone(data)
	}
	return nil
}

func decodeObject(data []byte, out any, strict bool) error {
	documents, err := yamlDocuments(data)
	if err != nil {
		return err
	}
	if len(documents) != 1 {
		return errors.New("must contain exactly one YAML object")
	}
	decoder := json.NewDecoder(bytes.NewReader(documents[0]))
	if strict {
		decoder.DisallowUnknownFields()
	}
	return decoder.Decode(out)
}

func yamlDocuments(data []byte) ([][]byte, error) {
	reader := yamlutil.NewYAMLReader(bufio.NewReader(bytes.NewReader(data)))
	var documents [][]byte
	for {
		document, err := reader.Read()
		if errors.Is(err, io.EOF) {
			return documents, nil
		}
		if err != nil {
			return nil, err
		}
		raw, err := yaml.YAMLToJSONStrict(document)
		if err != nil {
			return nil, err
		}
		if bytes.Equal(bytes.TrimSpace(raw), []byte("null")) {
			continue
		}
		var object map[string]json.RawMessage
		if err := json.Unmarshal(raw, &object); err != nil || object == nil {
			return nil, errors.New("must contain YAML objects")
		}
		documents = append(documents, raw)
	}
}

func validPointer(pointer string) bool {
	if !strings.HasPrefix(pointer, "/") {
		return false
	}
	for i := 0; i < len(pointer); i++ {
		if pointer[i] == '~' {
			i++
			if i == len(pointer) || (pointer[i] != '0' && pointer[i] != '1') {
				return false
			}
		}
	}
	return true
}
