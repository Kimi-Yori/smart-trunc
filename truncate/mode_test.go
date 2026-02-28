package truncate

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestModePatterns_General(t *testing.T) {
	patterns := ModePatterns("general")
	assert.NotEmpty(t, patterns)
	assert.Contains(t, patterns, `(?i)ERROR`)
	assert.Contains(t, patterns, `(?i)FATAL`)
	assert.Contains(t, patterns, `(?i)PANIC`)
}

func TestModePatterns_Test(t *testing.T) {
	patterns := ModePatterns("test")
	// test mode includes general patterns
	assert.Contains(t, patterns, `(?i)ERROR`)
	// plus test-specific patterns
	assert.Contains(t, patterns, `--- FAIL:`)
	assert.Contains(t, patterns, `(?i)FAIL`)
}

func TestModePatterns_Build(t *testing.T) {
	patterns := ModePatterns("build")
	// build mode includes general patterns
	assert.Contains(t, patterns, `(?i)ERROR`)
	// plus build-specific patterns
	assert.Contains(t, patterns, `npm ERR!`)
	assert.Contains(t, patterns, `(?i)syntax error`)
}

func TestModePatterns_UnknownFallsToGeneral(t *testing.T) {
	patterns := ModePatterns("unknown")
	generalPatterns := ModePatterns("general")
	assert.Equal(t, generalPatterns, patterns)
}
