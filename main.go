package main

import (
	"bufio"
	"flag"
	"fmt"
	"os"
	"strings"

	"github.com/Kimi-Yori/smart-trunc/truncate"
)

var version = "dev"

func main() {
	limit := flag.Int("limit", 30000, "output byte limit")
	head := flag.Int("head", 20, "lines to keep from head")
	tail := flag.Int("tail", 20, "lines to keep from tail")
	context := flag.Int("context", 3, "context lines around matched lines")
	mode := flag.String("mode", "general", "preset mode: general/test/build")
	jsonOut := flag.Bool("json", false, "JSON structured output")
	yamlOut := flag.Bool("yaml", false, "YAML structured output")
	showVersion := flag.Bool("version", false, "show version")

	var keepPatterns multiFlag
	flag.Var(&keepPatterns, "keep-pattern", "additional keep pattern (regexp, repeatable)")
	flag.Var(&keepPatterns, "k", "shorthand for --keep-pattern")

	flag.Usage = func() {
		fmt.Fprintf(os.Stderr, "Usage: smart-trunc [options]\n\n")
		fmt.Fprintf(os.Stderr, "Intelligent output truncation for LLM agents.\n")
		fmt.Fprintf(os.Stderr, "Pipe command output through smart-trunc to keep errors and key lines\n")
		fmt.Fprintf(os.Stderr, "within a byte limit. Short output passes through unchanged.\n\n")
		fmt.Fprintf(os.Stderr, "Options:\n")
		flag.PrintDefaults()
		fmt.Fprintf(os.Stderr, "\nModes:\n")
		fmt.Fprintf(os.Stderr, "  general  ERROR/FATAL/WARN etc. are prioritized (default)\n")
		fmt.Fprintf(os.Stderr, "  test     Remove PASS lines, protect summary lines (Go/pytest/jest)\n")
		fmt.Fprintf(os.Stderr, "  build    Build errors and warnings are prioritized\n")
		fmt.Fprintf(os.Stderr, "\nExamples:\n")
		fmt.Fprintf(os.Stderr, "  pytest -v 2>&1 | smart-trunc --mode test\n")
		fmt.Fprintf(os.Stderr, "  npm run build 2>&1 | smart-trunc --mode build\n")
		fmt.Fprintf(os.Stderr, "  any-command 2>&1 | smart-trunc\n")
	}

	flag.Parse()

	if *showVersion {
		fmt.Println("smart-trunc", version)
		os.Exit(0)
	}

	// Fix #6: --json and --yaml are mutually exclusive
	if *jsonOut && *yamlOut {
		fmt.Fprintln(os.Stderr, "smart-trunc: --json and --yaml are mutually exclusive")
		os.Exit(2)
	}

	if *head < 0 || *tail < 0 || *context < 0 {
		fmt.Fprintln(os.Stderr, "smart-trunc: --head, --tail, --context must be non-negative")
		os.Exit(2)
	}

	// Fix #1: return error on scanner failure (e.g. lines > 8MB)
	lines, err := readStdin()
	if err != nil {
		fmt.Fprintf(os.Stderr, "smart-trunc: %v\n", err)
		os.Exit(1)
	}

	var format truncate.OutputFormat
	switch {
	case *jsonOut:
		format = truncate.FormatJSON
	case *yamlOut:
		format = truncate.FormatYAML
	default:
		format = truncate.FormatPlain
	}

	opts := truncate.Options{
		Limit:        *limit,
		Head:         *head,
		Tail:         *tail,
		Context:      *context,
		Mode:         *mode,
		KeepPatterns: []string(keepPatterns),
		Format:       format,
	}

	result := truncate.Truncate(lines, opts)

	out, err := truncate.Render(result, format)
	if err != nil {
		fmt.Fprintf(os.Stderr, "smart-trunc: %v\n", err)
		os.Exit(1)
	}
	fmt.Print(out)
}

// readStdin reads all lines from stdin with an 8MB per-line buffer.
func readStdin() ([]string, error) {
	scanner := bufio.NewScanner(os.Stdin)
	const maxLineSize = 8 * 1024 * 1024 // 8MB
	scanner.Buffer(make([]byte, 64*1024), maxLineSize)

	lines := make([]string, 0, 1024)
	for scanner.Scan() {
		lines = append(lines, scanner.Text())
	}
	if err := scanner.Err(); err != nil {
		return lines, fmt.Errorf("read stdin: %w", err)
	}
	return lines, nil
}

// multiFlag allows repeatable flag values like -k "A" -k "B".
type multiFlag []string

func (f *multiFlag) String() string {
	return strings.Join(*f, ", ")
}

func (f *multiFlag) Set(value string) error {
	*f = append(*f, value)
	return nil
}
