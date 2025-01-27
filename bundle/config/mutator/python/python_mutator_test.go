package python

import (
	"context"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"reflect"
	"runtime"
	"testing"

	"github.com/databricks/cli/bundle/env"
	"github.com/stretchr/testify/require"

	"golang.org/x/exp/maps"

	"github.com/databricks/cli/libs/dyn"

	"github.com/databricks/cli/bundle"
	"github.com/databricks/cli/bundle/config"
	assert "github.com/databricks/cli/libs/dyn/dynassert"
	"github.com/databricks/cli/libs/process"
)

func TestPythonMutator_Name_load(t *testing.T) {
	mutator := PythonMutator(PythonMutatorPhaseLoad)

	assert.Equal(t, "PythonMutator(load)", mutator.Name())
}

func TestPythonMutator_Name_init(t *testing.T) {
	mutator := PythonMutator(PythonMutatorPhaseInit)

	assert.Equal(t, "PythonMutator(init)", mutator.Name())
}

func TestPythonMutator_load(t *testing.T) {
	withFakeVEnv(t, ".venv")

	b := loadYaml("databricks.yml", `
      experimental:
        pydabs:
          enabled: true
          venv_path: .venv
      resources:
        jobs:
          job0:
            name: job_0`)

	ctx := withProcessStub(
		t,
		[]string{
			interpreterPath(".venv"),
			"-m",
			"databricks.bundles.build",
			"--phase",
			"load",
		},
		`{
			"experimental": {
				"pydabs": {
					"enabled": true,
					"venv_path": ".venv"
				}
			},
			"resources": {
				"jobs": {
					"job0": {
						name: "job_0"
					},
					"job1": {
						name: "job_1"
					},
				}
			}
		}`)

	mutator := PythonMutator(PythonMutatorPhaseLoad)
	diag := bundle.Apply(ctx, b, mutator)

	assert.NoError(t, diag.Error())

	assert.ElementsMatch(t, []string{"job0", "job1"}, maps.Keys(b.Config.Resources.Jobs))

	if job0, ok := b.Config.Resources.Jobs["job0"]; ok {
		assert.Equal(t, "job_0", job0.Name)
	}

	if job1, ok := b.Config.Resources.Jobs["job1"]; ok {
		assert.Equal(t, "job_1", job1.Name)
	}
}

func TestPythonMutator_load_disallowed(t *testing.T) {
	withFakeVEnv(t, ".venv")

	b := loadYaml("databricks.yml", `
      experimental:
        pydabs:
          enabled: true
          venv_path: .venv
      resources:
        jobs:
          job0:
            name: job_0`)

	ctx := withProcessStub(
		t,
		[]string{
			interpreterPath(".venv"),
			"-m",
			"databricks.bundles.build",
			"--phase",
			"load",
		},
		`{
			"experimental": {
				"pydabs": {
					"enabled": true,
					"venv_path": ".venv"
				}
			},
			"resources": {
				"jobs": {
					"job0": {
						name: "job_0",
						description: "job description"
					}
				}
			}
		}`)

	mutator := PythonMutator(PythonMutatorPhaseLoad)
	diag := bundle.Apply(ctx, b, mutator)

	assert.EqualError(t, diag.Error(), "unexpected change at \"resources.jobs.job0.description\" (insert)")
}

func TestPythonMutator_init(t *testing.T) {
	withFakeVEnv(t, ".venv")

	b := loadYaml("databricks.yml", `
      experimental:
        pydabs:
          enabled: true
          venv_path: .venv
      resources:
        jobs:
          job0:
            name: job_0`)

	ctx := withProcessStub(
		t,
		[]string{
			interpreterPath(".venv"),
			"-m",
			"databricks.bundles.build",
			"--phase",
			"init",
		},
		`{
			"experimental": {
				"pydabs": {
					"enabled": true,
					"venv_path": ".venv"
				}
			},
			"resources": {
				"jobs": {
					"job0": {
						name: "job_0",
						description: "my job"
					}
				}
			}
		}`)

	mutator := PythonMutator(PythonMutatorPhaseInit)
	diag := bundle.Apply(ctx, b, mutator)

	assert.NoError(t, diag.Error())

	assert.ElementsMatch(t, []string{"job0"}, maps.Keys(b.Config.Resources.Jobs))
	assert.Equal(t, "job_0", b.Config.Resources.Jobs["job0"].Name)
	assert.Equal(t, "my job", b.Config.Resources.Jobs["job0"].Description)

	err := b.Config.Mutate(func(v dyn.Value) (dyn.Value, error) {
		// 'name' wasn't changed, so it keeps its location
		name, err := dyn.GetByPath(v, dyn.MustPathFromString("resources.jobs.job0.name"))
		require.NoError(t, err)
		assert.Equal(t, "databricks.yml", name.Location().File)

		// 'description' was updated by PyDABs and has location of generated file until
		// we implement source maps
		description, err := dyn.GetByPath(v, dyn.MustPathFromString("resources.jobs.job0.description"))
		require.NoError(t, err)

		expectedVirtualPath, err := filepath.Abs("__generated_by_pydabs__.yml")
		require.NoError(t, err)
		assert.Equal(t, expectedVirtualPath, description.Location().File)

		return v, nil
	})
	assert.NoError(t, err)
}

func TestPythonMutator_badOutput(t *testing.T) {
	withFakeVEnv(t, ".venv")

	b := loadYaml("databricks.yml", `
      experimental:
        pydabs:
          enabled: true
          venv_path: .venv
      resources:
        jobs:
          job0:
            name: job_0`)

	ctx := withProcessStub(
		t,
		[]string{
			interpreterPath(".venv"),
			"-m",
			"databricks.bundles.build",
			"--phase",
			"load",
		},
		`{
			"resources": {
				"jobs": {
					"job0": {
						unknown_property: "my job"
					}
				}
			}
		}`)

	mutator := PythonMutator(PythonMutatorPhaseLoad)
	diag := bundle.Apply(ctx, b, mutator)

	assert.EqualError(t, diag.Error(), "failed to normalize Python mutator output: unknown field: unknown_property")
}

func TestPythonMutator_disabled(t *testing.T) {
	b := loadYaml("databricks.yml", ``)

	ctx := context.Background()
	mutator := PythonMutator(PythonMutatorPhaseLoad)
	diag := bundle.Apply(ctx, b, mutator)

	assert.NoError(t, diag.Error())
}

func TestPythonMutator_venvRequired(t *testing.T) {
	b := loadYaml("databricks.yml", `
      experimental:
        pydabs:
          enabled: true`)

	ctx := context.Background()
	mutator := PythonMutator(PythonMutatorPhaseLoad)
	diag := bundle.Apply(ctx, b, mutator)

	assert.Error(t, diag.Error(), "\"experimental.enable_pydabs\" is enabled, but \"experimental.venv.path\" is not set")
}

func TestPythonMutator_venvNotFound(t *testing.T) {
	expectedError := fmt.Sprintf("can't find %q, check if venv is created", interpreterPath("bad_path"))

	b := loadYaml("databricks.yml", `
      experimental:
        pydabs:
          enabled: true
          venv_path: bad_path`)

	mutator := PythonMutator(PythonMutatorPhaseInit)
	diag := bundle.Apply(context.Background(), b, mutator)

	assert.EqualError(t, diag.Error(), expectedError)
}

type createOverrideVisitorTestCase struct {
	name        string
	updatePath  dyn.Path
	deletePath  dyn.Path
	insertPath  dyn.Path
	phase       phase
	updateError error
	deleteError error
	insertError error
}

func TestCreateOverrideVisitor(t *testing.T) {
	left := dyn.NewValue(42, dyn.Location{})
	right := dyn.NewValue(1337, dyn.Location{})

	testCases := []createOverrideVisitorTestCase{
		{
			name:        "load: can't change an existing job",
			phase:       PythonMutatorPhaseLoad,
			updatePath:  dyn.MustPathFromString("resources.jobs.job0.name"),
			deletePath:  dyn.MustPathFromString("resources.jobs.job0.name"),
			insertPath:  dyn.MustPathFromString("resources.jobs.job0.name"),
			deleteError: fmt.Errorf("unexpected change at \"resources.jobs.job0.name\" (delete)"),
			insertError: fmt.Errorf("unexpected change at \"resources.jobs.job0.name\" (insert)"),
			updateError: fmt.Errorf("unexpected change at \"resources.jobs.job0.name\" (update)"),
		},
		{
			name:        "load: can't delete an existing job",
			phase:       PythonMutatorPhaseLoad,
			deletePath:  dyn.MustPathFromString("resources.jobs.job0"),
			deleteError: fmt.Errorf("unexpected change at \"resources.jobs.job0\" (delete)"),
		},
		{
			name:        "load: can insert a job",
			phase:       PythonMutatorPhaseLoad,
			insertPath:  dyn.MustPathFromString("resources.jobs.job0"),
			insertError: nil,
		},
		{
			name:        "load: can't change include",
			phase:       PythonMutatorPhaseLoad,
			deletePath:  dyn.MustPathFromString("include[0]"),
			insertPath:  dyn.MustPathFromString("include[0]"),
			updatePath:  dyn.MustPathFromString("include[0]"),
			deleteError: fmt.Errorf("unexpected change at \"include[0]\" (delete)"),
			insertError: fmt.Errorf("unexpected change at \"include[0]\" (insert)"),
			updateError: fmt.Errorf("unexpected change at \"include[0]\" (update)"),
		},
		{
			name:        "init: can change an existing job",
			phase:       PythonMutatorPhaseInit,
			updatePath:  dyn.MustPathFromString("resources.jobs.job0.name"),
			deletePath:  dyn.MustPathFromString("resources.jobs.job0.name"),
			insertPath:  dyn.MustPathFromString("resources.jobs.job0.name"),
			deleteError: nil,
			insertError: nil,
			updateError: nil,
		},
		{
			name:        "init: can't delete an existing job",
			phase:       PythonMutatorPhaseInit,
			deletePath:  dyn.MustPathFromString("resources.jobs.job0"),
			deleteError: fmt.Errorf("unexpected change at \"resources.jobs.job0\" (delete)"),
		},
		{
			name:        "init: can insert a job",
			phase:       PythonMutatorPhaseInit,
			insertPath:  dyn.MustPathFromString("resources.jobs.job0"),
			insertError: nil,
		},
		{
			name:        "init: can't change include",
			phase:       PythonMutatorPhaseInit,
			deletePath:  dyn.MustPathFromString("include[0]"),
			insertPath:  dyn.MustPathFromString("include[0]"),
			updatePath:  dyn.MustPathFromString("include[0]"),
			deleteError: fmt.Errorf("unexpected change at \"include[0]\" (delete)"),
			insertError: fmt.Errorf("unexpected change at \"include[0]\" (insert)"),
			updateError: fmt.Errorf("unexpected change at \"include[0]\" (update)"),
		},
	}

	for _, tc := range testCases {
		visitor, err := createOverrideVisitor(context.Background(), tc.phase)
		if err != nil {
			t.Fatalf("create visitor failed: %v", err)
		}

		if tc.updatePath != nil {
			t.Run(tc.name+"-update", func(t *testing.T) {
				out, err := visitor.VisitUpdate(tc.updatePath, left, right)

				if tc.updateError != nil {
					assert.Equal(t, tc.updateError, err)
				} else {
					assert.NoError(t, err)
					assert.Equal(t, right, out)
				}
			})
		}

		if tc.deletePath != nil {
			t.Run(tc.name+"-delete", func(t *testing.T) {
				err := visitor.VisitDelete(tc.deletePath, left)

				if tc.deleteError != nil {
					assert.Equal(t, tc.deleteError, err)
				} else {
					assert.NoError(t, err)
				}
			})
		}

		if tc.insertPath != nil {
			t.Run(tc.name+"-insert", func(t *testing.T) {
				out, err := visitor.VisitInsert(tc.insertPath, right)

				if tc.insertError != nil {
					assert.Equal(t, tc.insertError, err)
				} else {
					assert.NoError(t, err)
					assert.Equal(t, right, out)
				}
			})
		}
	}
}

func TestInterpreterPath(t *testing.T) {
	if runtime.GOOS == "windows" {
		assert.Equal(t, "venv\\Scripts\\python3.exe", interpreterPath("venv"))
	} else {
		assert.Equal(t, "venv/bin/python3", interpreterPath("venv"))
	}
}

func withProcessStub(t *testing.T, args []string, stdout string) context.Context {
	ctx := context.Background()
	ctx, stub := process.WithStub(ctx)

	t.Setenv(env.TempDirVariable, t.TempDir())

	// after we override env variable, we always get the same cache dir as mutator
	cacheDir, err := createCacheDir(ctx)
	require.NoError(t, err)

	inputPath := filepath.Join(cacheDir, "input.json")
	outputPath := filepath.Join(cacheDir, "output.json")

	args = append(args, "--input", inputPath)
	args = append(args, "--output", outputPath)

	stub.WithCallback(func(actual *exec.Cmd) error {
		_, err := os.Stat(inputPath)
		assert.NoError(t, err)

		if reflect.DeepEqual(actual.Args, args) {
			err := os.WriteFile(outputPath, []byte(stdout), 0600)
			return err
		} else {
			return fmt.Errorf("unexpected command: %v", actual.Args)
		}
	})

	return ctx
}

func loadYaml(name string, content string) *bundle.Bundle {
	v, diag := config.LoadFromBytes(name, []byte(content))

	if diag.Error() != nil {
		panic(diag.Error())
	}

	return &bundle.Bundle{
		Config: *v,
	}
}

func withFakeVEnv(t *testing.T, path string) {
	interpreterPath := interpreterPath(path)

	cwd, err := os.Getwd()
	if err != nil {
		panic(err)
	}

	if err := os.Chdir(t.TempDir()); err != nil {
		panic(err)
	}

	err = os.MkdirAll(filepath.Dir(interpreterPath), 0755)
	if err != nil {
		panic(err)
	}

	err = os.WriteFile(interpreterPath, []byte(""), 0755)
	if err != nil {
		panic(err)
	}

	t.Cleanup(func() {
		if err := os.Chdir(cwd); err != nil {
			panic(err)
		}
	})
}
