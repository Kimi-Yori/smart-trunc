package truncate

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// goldenTest runs a golden test: read input, truncate, compare with expected output.
// If UPDATE_GOLDEN=1, writes the actual output as the new golden file.
func goldenTest(t *testing.T, dir string, opts Options, format OutputFormat, goldenName string) {
	t.Helper()

	inputPath := filepath.Join("..", "testdata", dir, "input.txt")
	goldenPath := filepath.Join("..", "testdata", dir, goldenName)

	inputBytes, err := os.ReadFile(inputPath)
	require.NoError(t, err, "failed to read input file")

	lines := strings.Split(strings.TrimRight(string(inputBytes), "\n"), "\n")
	result := Truncate(lines, opts)
	actual, err := Render(result, format)
	require.NoError(t, err)

	if os.Getenv("UPDATE_GOLDEN") == "1" {
		err := os.WriteFile(goldenPath, []byte(actual), 0644)
		require.NoError(t, err, "failed to write golden file")
		t.Logf("updated golden file: %s", goldenPath)
		return
	}

	goldenBytes, err := os.ReadFile(goldenPath)
	if os.IsNotExist(err) {
		t.Fatalf("golden file not found: %s (run with UPDATE_GOLDEN=1 to create)", goldenPath)
	}
	require.NoError(t, err)

	assert.Equal(t, string(goldenBytes), actual, "output differs from golden file %s", goldenPath)
}

func TestGolden_PytestFailure_Plain(t *testing.T) {
	goldenTest(t, "pytest_failure", Options{
		Limit:   500,
		Head:    5,
		Tail:    5,
		Context: 2,
		Mode:    "test",
	}, FormatPlain, "output_plain.golden")
}

func TestGolden_NpmBuildError_Plain(t *testing.T) {
	goldenTest(t, "npm_build_error", Options{
		Limit:   500,
		Head:    5,
		Tail:    5,
		Context: 2,
		Mode:    "build",
	}, FormatPlain, "output_plain.golden")
}

func TestGolden_GoTestFailure_Plain(t *testing.T) {
	goldenTest(t, "go_test_failure", Options{
		Limit:   400,
		Head:    3,
		Tail:    3,
		Context: 1,
		Mode:    "test",
	}, FormatPlain, "output_plain.golden")
}

func TestGolden_GoTestAllPass_Plain(t *testing.T) {
	goldenTest(t, "go_test_allpass", Options{
		Limit:   30000,
		Head:    20,
		Tail:    20,
		Context: 3,
		Mode:    "test",
	}, FormatPlain, "output_plain.golden")
}

func TestGolden_PytestAllPass_Plain(t *testing.T) {
	goldenTest(t, "pytest_allpass", Options{
		Limit:   30000,
		Head:    20,
		Tail:    20,
		Context: 3,
		Mode:    "test",
	}, FormatPlain, "output_plain.golden")
}

func TestGolden_JestAllPass_Plain(t *testing.T) {
	goldenTest(t, "jest_allpass", Options{
		Limit:   30000,
		Head:    20,
		Tail:    20,
		Context: 3,
		Mode:    "test",
	}, FormatPlain, "output_plain.golden")
}
