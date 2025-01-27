package python

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"runtime"

	"github.com/databricks/cli/bundle/env"

	"github.com/databricks/cli/bundle"
	"github.com/databricks/cli/bundle/config"
	"github.com/databricks/cli/libs/diag"
	"github.com/databricks/cli/libs/dyn"
	"github.com/databricks/cli/libs/dyn/convert"
	"github.com/databricks/cli/libs/dyn/merge"
	"github.com/databricks/cli/libs/dyn/yamlloader"
	"github.com/databricks/cli/libs/log"
	"github.com/databricks/cli/libs/process"
)

type phase string

const (
	// PythonMutatorPhaseLoad is the phase in which bundle configuration is loaded.
	//
	// At this stage, PyDABs adds statically defined resources to the bundle configuration.
	// Which resources are added should be deterministic and not depend on the bundle configuration.
	//
	// We also open for possibility of appending other sections of bundle configuration,
	// for example, adding new variables. However, this is not supported yet, and CLI rejects
	// such changes.
	PythonMutatorPhaseLoad phase = "load"

	// PythonMutatorPhaseInit is the phase after bundle configuration was loaded, and
	// the list of statically declared resources is known.
	//
	// At this stage, PyDABs adds resources defined using generators, or mutates existing resources,
	// including the ones defined using YAML.
	//
	// During this process, within generator and mutators, PyDABs can access:
	// - selected deployment target
	// - bundle variables values
	// - variables provided through CLI arguments or environment variables
	//
	// The following is not available:
	// - variables referencing other variables are in unresolved format
	//
	// PyDABs can output YAML containing references to variables, and CLI should resolve them.
	//
	// Existing resources can't be removed, and CLI rejects such changes.
	PythonMutatorPhaseInit phase = "init"
)

type pythonMutator struct {
	phase phase
}

func PythonMutator(phase phase) bundle.Mutator {
	return &pythonMutator{
		phase: phase,
	}
}

func (m *pythonMutator) Name() string {
	return fmt.Sprintf("PythonMutator(%s)", m.phase)
}

func getExperimental(b *bundle.Bundle) config.Experimental {
	if b.Config.Experimental == nil {
		return config.Experimental{}
	}

	return *b.Config.Experimental
}

func (m *pythonMutator) Apply(ctx context.Context, b *bundle.Bundle) diag.Diagnostics {
	experimental := getExperimental(b)

	if !experimental.PyDABs.Enabled {
		return nil
	}

	if experimental.PyDABs.VEnvPath == "" {
		return diag.Errorf("\"experimental.pydabs.enabled\" can only be used when \"experimental.pydabs.venv_path\" is set")
	}

	err := b.Config.Mutate(func(leftRoot dyn.Value) (dyn.Value, error) {
		pythonPath := interpreterPath(experimental.PyDABs.VEnvPath)

		if _, err := os.Stat(pythonPath); err != nil {
			if os.IsNotExist(err) {
				return dyn.InvalidValue, fmt.Errorf("can't find %q, check if venv is created", pythonPath)
			} else {
				return dyn.InvalidValue, fmt.Errorf("can't find %q: %w", pythonPath, err)
			}
		}

		cacheDir, err := createCacheDir(ctx)
		if err != nil {
			return dyn.InvalidValue, fmt.Errorf("failed to create cache dir: %w", err)
		}

		rightRoot, err := m.runPythonMutator(ctx, cacheDir, b.RootPath, pythonPath, leftRoot)
		if err != nil {
			return dyn.InvalidValue, err
		}

		visitor, err := createOverrideVisitor(ctx, m.phase)
		if err != nil {
			return dyn.InvalidValue, err
		}

		return merge.Override(leftRoot, rightRoot, visitor)
	})

	return diag.FromErr(err)
}

func createCacheDir(ctx context.Context) (string, error) {
	// b.CacheDir doesn't work because target isn't yet selected

	// support the same env variable as in b.CacheDir
	if tempDir, exists := env.TempDir(ctx); exists {
		// use 'default' as target name
		cacheDir := filepath.Join(tempDir, "default", "pydabs")

		err := os.MkdirAll(cacheDir, 0700)
		if err != nil {
			return "", err
		}

		return cacheDir, nil
	}

	return os.MkdirTemp("", "-pydabs")
}

func (m *pythonMutator) runPythonMutator(ctx context.Context, cacheDir string, rootPath string, pythonPath string, root dyn.Value) (dyn.Value, error) {
	inputPath := filepath.Join(cacheDir, "input.json")
	outputPath := filepath.Join(cacheDir, "output.json")

	args := []string{
		pythonPath,
		"-m",
		"databricks.bundles.build",
		"--phase",
		string(m.phase),
		"--input",
		inputPath,
		"--output",
		outputPath,
	}

	// we need to marshal dyn.Value instead of bundle.Config to JSON to support
	// non-string fields assigned with bundle variables
	rootConfigJson, err := json.Marshal(root.AsAny())
	if err != nil {
		return dyn.InvalidValue, fmt.Errorf("failed to marshal root config: %w", err)
	}

	err = os.WriteFile(inputPath, rootConfigJson, 0600)
	if err != nil {
		return dyn.InvalidValue, fmt.Errorf("failed to write input file: %w", err)
	}

	stderrWriter := newLogWriter(ctx, "stderr: ")
	stdoutWriter := newLogWriter(ctx, "stdout: ")

	_, err = process.Background(
		ctx,
		args,
		process.WithDir(rootPath),
		process.WithStderrWriter(stderrWriter),
		process.WithStdoutWriter(stdoutWriter),
	)
	if err != nil {
		return dyn.InvalidValue, fmt.Errorf("python mutator process failed: %w", err)
	}

	outputFile, err := os.Open(outputPath)
	if err != nil {
		return dyn.InvalidValue, fmt.Errorf("failed to open Python mutator output: %w", err)
	}

	defer func() {
		_ = outputFile.Close()
	}()

	// we need absolute path because later parts of pipeline assume all paths are absolute
	// and this file will be used as location to resolve relative paths.
	//
	// virtualPath has to stay in rootPath, because locations outside root path are not allowed:
	//
	//   Error: path /var/folders/.../pydabs/dist/*.whl is not contained in bundle root path
	//
	// for that, we pass virtualPath instead of outputPath as file location
	virtualPath, err := filepath.Abs(filepath.Join(rootPath, "__generated_by_pydabs__.yml"))
	if err != nil {
		return dyn.InvalidValue, fmt.Errorf("failed to get absolute path: %w", err)
	}

	generated, err := yamlloader.LoadYAML(virtualPath, outputFile)
	if err != nil {
		return dyn.InvalidValue, fmt.Errorf("failed to parse Python mutator output: %w", err)
	}

	normalized, diagnostic := convert.Normalize(config.Root{}, generated)
	if diagnostic.Error() != nil {
		return dyn.InvalidValue, fmt.Errorf("failed to normalize Python mutator output: %w", diagnostic.Error())
	}

	// warnings shouldn't happen because output should be already normalized
	// when it happens, it's a bug in the mutator, and should be treated as an error

	for _, d := range diagnostic.Filter(diag.Warning) {
		return dyn.InvalidValue, fmt.Errorf("failed to normalize Python mutator output: %s", d.Summary)
	}

	return normalized, nil
}

func createOverrideVisitor(ctx context.Context, phase phase) (merge.OverrideVisitor, error) {
	switch phase {
	case PythonMutatorPhaseLoad:
		return createLoadOverrideVisitor(ctx), nil
	case PythonMutatorPhaseInit:
		return createInitOverrideVisitor(ctx), nil
	default:
		return merge.OverrideVisitor{}, fmt.Errorf("unknown phase: %s", phase)
	}
}

// createLoadOverrideVisitor creates an override visitor for the load phase.
//
// During load, it's only possible to create new resources, and not modify or
// delete existing ones.
func createLoadOverrideVisitor(ctx context.Context) merge.OverrideVisitor {
	jobsPath := dyn.NewPath(dyn.Key("resources"), dyn.Key("jobs"))

	return merge.OverrideVisitor{
		VisitDelete: func(valuePath dyn.Path, left dyn.Value) error {
			return fmt.Errorf("unexpected change at %q (delete)", valuePath.String())
		},
		VisitInsert: func(valuePath dyn.Path, right dyn.Value) (dyn.Value, error) {
			if !valuePath.HasPrefix(jobsPath) {
				return dyn.InvalidValue, fmt.Errorf("unexpected change at %q (insert)", valuePath.String())
			}

			insertResource := len(valuePath) == len(jobsPath)+1

			// adding a property into an existing resource is not allowed, because it changes it
			if !insertResource {
				return dyn.InvalidValue, fmt.Errorf("unexpected change at %q (insert)", valuePath.String())
			}

			log.Debugf(ctx, "Insert value at %q", valuePath.String())

			return right, nil
		},
		VisitUpdate: func(valuePath dyn.Path, left dyn.Value, right dyn.Value) (dyn.Value, error) {
			return dyn.InvalidValue, fmt.Errorf("unexpected change at %q (update)", valuePath.String())
		},
	}
}

// createInitOverrideVisitor creates an override visitor for the init phase.
//
// During the init phase it's possible to create new resources, modify existing
// resources, but not delete existing resources.
func createInitOverrideVisitor(ctx context.Context) merge.OverrideVisitor {
	jobsPath := dyn.NewPath(dyn.Key("resources"), dyn.Key("jobs"))

	return merge.OverrideVisitor{
		VisitDelete: func(valuePath dyn.Path, left dyn.Value) error {
			if !valuePath.HasPrefix(jobsPath) {
				return fmt.Errorf("unexpected change at %q (delete)", valuePath.String())
			}

			deleteResource := len(valuePath) == len(jobsPath)+1

			if deleteResource {
				return fmt.Errorf("unexpected change at %q (delete)", valuePath.String())
			}

			// deleting properties is allowed because it only changes an existing resource
			log.Debugf(ctx, "Delete value at %q", valuePath.String())

			return nil
		},
		VisitInsert: func(valuePath dyn.Path, right dyn.Value) (dyn.Value, error) {
			if !valuePath.HasPrefix(jobsPath) {
				return dyn.InvalidValue, fmt.Errorf("unexpected change at %q (insert)", valuePath.String())
			}

			log.Debugf(ctx, "Insert value at %q", valuePath.String())

			return right, nil
		},
		VisitUpdate: func(valuePath dyn.Path, left dyn.Value, right dyn.Value) (dyn.Value, error) {
			if !valuePath.HasPrefix(jobsPath) {
				return dyn.InvalidValue, fmt.Errorf("unexpected change at %q (update)", valuePath.String())
			}

			log.Debugf(ctx, "Update value at %q", valuePath.String())

			return right, nil
		},
	}
}

// interpreterPath returns platform-specific path to Python interpreter in the virtual environment.
func interpreterPath(venvPath string) string {
	if runtime.GOOS == "windows" {
		return filepath.Join(venvPath, "Scripts", "python3.exe")
	} else {
		return filepath.Join(venvPath, "bin", "python3")
	}
}
