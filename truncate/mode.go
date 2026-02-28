package truncate

// ModePatterns returns the regex pattern strings for the given mode name.
func ModePatterns(mode string) []string {
	switch mode {
	case "test":
		return testPatterns()
	case "build":
		return buildPatterns()
	default:
		return generalPatterns()
	}
}

func generalPatterns() []string {
	return []string{
		`(?i)ERROR`,
		`(?i)FATAL`,
		`(?i)WARN`,
		`(?i)PANIC`,
		`(?i)traceback`,
		`(?i)exception`,
	}
}

func testPatterns() []string {
	return append(generalPatterns(),
		`(?i)FAIL`,
		`(?i)AssertionError`,
		`(?i)AssertionFailure`,
		`Expected.*Actual`,
		`--- FAIL:`,
		`(?i)test summary`,
	)
}

// testSummaryPatterns returns patterns that identify test summary lines.
// These lines are always kept in test mode.
func testSummaryPatterns() []string {
	return []string{
		`^ok\s+\S+`,              // Go: ok  github.com/...
		`^FAIL\s+\S+`,           // Go: FAIL github.com/...
		`\d+\s+passed`,          // pytest: 1 failed, 24 passed
		`\d+\s+failed`,          // pytest: 1 failed, 24 passed
		`Tests:\s+\d+`,          // jest: Tests: 2 failed, 18 passed
		`Test Suites:\s+\d+`,    // jest: Test Suites: 1 failed, 5 passed
	}
}

// testPassPatterns returns patterns that identify individual PASS lines.
// These lines are removed in test mode to save tokens.
func testPassPatterns() []string {
	return []string{
		`^--- PASS:`,            // Go: --- PASS: TestAdd (0.00s)
		`^=== RUN\s+`,          // Go: === RUN   TestAdd
		`\bPASSED\b`,           // pytest: test_add PASSED
		`^\s*[✓√]`,             // jest/vitest: ✓ should add (3 ms)
	}
}

func buildPatterns() []string {
	return append(generalPatterns(),
		`(?i)error:`,
		`(?i)warning:`,
		`(?i)fatal:`,
		`undefined reference`,
		`cannot find module`,
		`(?i)syntax error`,
		`npm ERR!`,
	)
}
