package truncate

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestBuildKeepMap_HeadAndTail(t *testing.T) {
	scores := make([]int, 10)
	keep := BuildKeepMap(10, scores, 3, 3, 0)

	// First 3 lines kept
	assert.True(t, keep[0])
	assert.True(t, keep[1])
	assert.True(t, keep[2])
	// Middle not kept
	assert.False(t, keep[3])
	assert.False(t, keep[6])
	// Last 3 lines kept
	assert.True(t, keep[7])
	assert.True(t, keep[8])
	assert.True(t, keep[9])
}

func TestBuildKeepMap_MatchWithContext(t *testing.T) {
	scores := make([]int, 10)
	scores[5] = 1 // line 5 matches

	keep := BuildKeepMap(10, scores, 0, 0, 2)

	assert.False(t, keep[2])
	assert.True(t, keep[3]) // context
	assert.True(t, keep[4]) // context
	assert.True(t, keep[5]) // match
	assert.True(t, keep[6]) // context
	assert.True(t, keep[7]) // context
	assert.False(t, keep[8])
}

func TestBuildKeepMap_ShortInput(t *testing.T) {
	scores := make([]int, 3)
	keep := BuildKeepMap(3, scores, 20, 20, 3)
	// All lines should be kept since head+tail > lineCount
	for _, k := range keep {
		assert.True(t, k)
	}
}

func TestBuildKeepMap_EmptyInput(t *testing.T) {
	keep := BuildKeepMap(0, nil, 5, 5, 3)
	assert.Empty(t, keep)
}

func TestBuildBlocks_AllKept(t *testing.T) {
	lines := []string{"a", "b", "c"}
	scores := []int{0, 0, 0}
	keep := []bool{true, true, true}

	blocks := BuildBlocks(lines, keep, scores, 3, 0)
	assert.Len(t, blocks, 1)
	assert.Equal(t, BlockHead, blocks[0].Type)
	assert.Equal(t, "a\nb\nc", blocks[0].Content)
}

func TestBuildBlocks_WithOmission(t *testing.T) {
	lines := []string{"head1", "head2", "mid1", "mid2", "mid3", "tail1", "tail2"}
	scores := []int{0, 0, 0, 0, 0, 0, 0}
	keep := []bool{true, true, false, false, false, true, true}

	blocks := BuildBlocks(lines, keep, scores, 2, 2)

	assert.Len(t, blocks, 3)
	assert.Equal(t, BlockHead, blocks[0].Type)
	assert.Equal(t, "head1\nhead2", blocks[0].Content)
	assert.Equal(t, BlockOmitted, blocks[1].Type)
	assert.Equal(t, 3, blocks[1].OmittedCount)
	assert.Equal(t, BlockTail, blocks[2].Type)
	assert.Equal(t, "tail1\ntail2", blocks[2].Content)
}

func TestBuildBlocks_MatchBlock(t *testing.T) {
	lines := []string{"ok", "ok", "ERROR here", "ok", "ok"}
	scores := []int{0, 0, 1, 0, 0}
	keep := []bool{false, true, true, true, false}

	blocks := BuildBlocks(lines, keep, scores, 0, 0)

	assert.Len(t, blocks, 3)
	assert.Equal(t, BlockOmitted, blocks[0].Type)
	assert.Equal(t, BlockMatch, blocks[1].Type)
	assert.Contains(t, blocks[1].Content, "ERROR here")
	assert.Equal(t, BlockOmitted, blocks[2].Type)
}

func TestBuildBlocks_EmptyInput(t *testing.T) {
	blocks := BuildBlocks(nil, nil, nil, 5, 5)
	assert.Nil(t, blocks)
}

// Issue 2 (5th review): negative context must be clamped to 0
func TestBuildKeepMap_NegativeContextClamped(t *testing.T) {
	scores := []int{0, 0, 1, 0, 0}
	keep := BuildKeepMap(5, scores, 0, 0, -1)
	assert.True(t, keep[2], "match line must be kept even with negative context")
}
