// Package truncate provides intelligent text truncation for LLM agent output.
package truncate

import (
	"fmt"
	"sort"
	"strings"
)

// Options configures the truncation behavior.
type Options struct {
	Limit        int
	Head         int
	Tail         int
	Context      int
	Mode         string
	KeepPatterns []string
	Format       OutputFormat
}

// OutputFormat selects the rendering format.
type OutputFormat int

const (
	FormatPlain OutputFormat = iota
	FormatJSON
	FormatYAML
)

// Result holds the truncation output.
type Result struct {
	TotalLines      int          `json:"total_lines" yaml:"total_lines"`
	KeptLines       int          `json:"kept_lines" yaml:"kept_lines"`
	OmittedLines    int          `json:"omitted_lines" yaml:"omitted_lines"`
	PatternsMatched int          `json:"patterns_matched" yaml:"patterns_matched"`
	Blocks          []Block      `json:"blocks" yaml:"blocks"`
	EffectiveLimit  int          `json:"-" yaml:"-"` // internal: byte limit for Render hard-cut
	LineScores      []int        `json:"-" yaml:"-"` // internal: per-line importance scores for block reduction
}

// Truncate applies intelligent truncation to the input lines.
func Truncate(lines []string, opts Options) Result {
	total := len(lines)
	if total == 0 {
		return Result{}
	}

	// Always score for PatternsMatched (even in short-circuit/unlimited paths).
	scorer, _ := NewScorer(opts.Mode, opts.KeepPatterns)
	scores := scorer.ScoreLines(lines)
	patternsMatched := 0
	for _, s := range scores {
		if s > 0 {
			patternsMatched++
		}
	}

	// Unlimited mode: limit<=0 means no truncation.
	if opts.Limit <= 0 {
		r := shortCircuit(lines, total, 0, patternsMatched, scores, opts.Head, opts.Tail)
		r.LineScores = scores
		return r
	}

	// Compute effective limit based on output format.
	effectiveLimit := effectiveLimitForFormat(opts.Limit, opts.Format)

	// Short-circuit: if rendered plain output fits within effective limit, pass through
	// For test mode, we still need to apply TestFilter even in short-circuit path
	if renderedPlainBytes(lines) <= effectiveLimit {
		if opts.Mode == "test" {
			// Fall through to normal path for test filtering
		} else {
			r := shortCircuit(lines, total, effectiveLimit, patternsMatched, scores, opts.Head, opts.Tail)
			r.LineScores = scores
			return r
		}
	}

	keep := BuildKeepMap(total, scores, opts.Head, opts.Tail, opts.Context)

	// Test mode: filter out PASS lines and protect summary lines
	if opts.Mode == "test" {
		tf := NewTestFilter(lines, scores)
		keep = tf.Apply(keep, lines, scores)
	}

	// Enforce limit, dropping anchors if needed
	enfOpts := opts
	enfOpts.Limit = effectiveLimit
	keep = enforceCharLimit(lines, scores, keep, enfOpts)

	blocks := BuildBlocks(lines, keep, scores, opts.Head, opts.Tail)

	keptLines := 0
	for _, k := range keep {
		if k {
			keptLines++
		}
	}

	return Result{
		TotalLines:      total,
		KeptLines:       keptLines,
		OmittedLines:    total - keptLines,
		PatternsMatched: patternsMatched,
		Blocks:          blocks,
		EffectiveLimit:  effectiveLimit,
		LineScores:      scores,
	}
}

// effectiveLimitForFormat returns the byte budget for truncation based on format.
// Plain text: 90% of limit (content only, no metadata overhead).
// YAML/JSON: full limit (metadata is part of the output).
func effectiveLimitForFormat(limit int, format OutputFormat) int {
	if limit <= 0 {
		return 0
	}
	switch format {
	case FormatJSON, FormatYAML:
		return limit
	default:
		v := int(float64(limit) * 0.9)
		if v < 1 {
			return 1
		}
		return v
	}
}

// renderedPlainBytes calculates the byte size of lines as renderPlain would output them.
// renderPlain outputs: content + "\n" per block. For a single short-circuit block,
// this equals: join(lines, "\n") + trailing "\n".
func renderedPlainBytes(lines []string) int {
	n := 0
	for i, s := range lines {
		n += len(s)
		if i+1 < len(lines) {
			n++ // newline separator between lines
		}
	}
	n++ // trailing newline from renderPlain
	return n
}

// shortCircuit returns a passthrough result when no truncation is needed.
// Block type is determined by classifyBlock so that if structured output
// later needs reduction, the block priority is correct (e.g. BlockMatch
// preserves the match line instead of BlockHead keeping only the start).
func shortCircuit(lines []string, total int, effectiveLimit int, patternsMatched int, scores []int, head int, tail int) Result {
	blockType := classifyBlock(0, total-1, total, scores, head, tail)
	return Result{
		TotalLines:      total,
		KeptLines:       total,
		OmittedLines:    0,
		PatternsMatched: patternsMatched,
		Blocks: []Block{
			{
				Type:      blockType,
				StartLine: 1,
				EndLine:   total,
				Content:   strings.Join(lines, "\n"),
			},
		},
		EffectiveLimit: effectiveLimit,
	}
}

// enforceCharLimit removes lines until the output fits within the byte limit.
// Uses sorted indices + union-find for O(n log n) total.
// Pass 1: drop lowest-score non-anchor lines.
// Pass 2: if still over, drop anchor lines too.
func enforceCharLimit(lines []string, scores []int, keep []bool, opts Options) []bool {
	total := len(lines)
	currentBytes := estimateOutputBytes(lines, keep, total)
	if currentBytes <= opts.Limit {
		return keep
	}

	// Build sorted index lists (ascending by score, then by line index)
	var nonAnchors, anchors []int
	for i := 0; i < total; i++ {
		if !keep[i] {
			continue
		}
		if i < opts.Head || i >= total-opts.Tail {
			anchors = append(anchors, i)
		} else {
			nonAnchors = append(nonAnchors, i)
		}
	}

	sortByScoreAsc := func(indices []int) {
		sort.Slice(indices, func(a, b int) bool {
			ia, ib := indices[a], indices[b]
			if scores[ia] != scores[ib] {
				return scores[ia] < scores[ib]
			}
			return ia < ib
		})
	}
	sortByScoreAsc(nonAnchors)
	sortByScoreAsc(anchors)

	// Initialize union-find for omitted regions
	uf := newUnionFind(total)
	for i := 0; i < total; i++ {
		if !keep[i] {
			uf.makeRegion(i)
			if i > 0 && !keep[i-1] {
				uf.union(i-1, i)
			}
		}
	}

	// dropLine drops a line and returns the byte delta (negative = savings)
	dropLine := func(idx int) int {
		keep[idx] = false
		lineBytes := len(lines[idx]) + 1

		leftOmitted := idx > 0 && !keep[idx-1]
		rightOmitted := idx+1 < total && !keep[idx+1]

		uf.makeRegion(idx)

		if leftOmitted && rightOmitted {
			leftSize := uf.regionSize(idx - 1)
			rightSize := uf.regionSize(idx + 1)
			oldCost := omittedMarkerLen(leftSize) + omittedMarkerLen(rightSize)
			uf.union(idx-1, idx)
			uf.union(idx, idx+1)
			newSize := uf.regionSize(idx)
			newCost := omittedMarkerLen(newSize)
			return -lineBytes - oldCost + newCost
		} else if leftOmitted {
			oldCost := omittedMarkerLen(uf.regionSize(idx - 1))
			uf.union(idx-1, idx)
			newCost := omittedMarkerLen(uf.regionSize(idx))
			return -lineBytes - oldCost + newCost
		} else if rightOmitted {
			oldCost := omittedMarkerLen(uf.regionSize(idx + 1))
			uf.union(idx, idx+1)
			newCost := omittedMarkerLen(uf.regionSize(idx))
			return -lineBytes - oldCost + newCost
		} else {
			newCost := omittedMarkerLen(1)
			return -lineBytes + newCost
		}
	}

	// Pass 1: drop non-anchors
	for _, idx := range nonAnchors {
		if currentBytes <= opts.Limit {
			break
		}
		currentBytes += dropLine(idx)
	}

	// Pass 2: drop anchors
	for _, idx := range anchors {
		if currentBytes <= opts.Limit {
			break
		}
		currentBytes += dropLine(idx)
	}

	return keep
}

// unionFind tracks omitted regions for O(α(n)) ≈ O(1) size queries.
type unionFind struct {
	parent []int
	size   []int
	active []bool // whether this position is part of an omitted region
}

func newUnionFind(n int) *unionFind {
	parent := make([]int, n)
	size := make([]int, n)
	active := make([]bool, n)
	for i := range parent {
		parent[i] = i
		size[i] = 1
	}
	return &unionFind{parent: parent, size: size, active: active}
}

func (u *unionFind) makeRegion(i int) {
	u.active[i] = true
}

func (u *unionFind) find(i int) int {
	for u.parent[i] != i {
		u.parent[i] = u.parent[u.parent[i]] // path compression
		i = u.parent[i]
	}
	return i
}

func (u *unionFind) union(a, b int) {
	ra, rb := u.find(a), u.find(b)
	if ra == rb {
		return
	}
	// Union by size
	if u.size[ra] < u.size[rb] {
		ra, rb = rb, ra
	}
	u.parent[rb] = ra
	u.size[ra] += u.size[rb]
}

func (u *unionFind) regionSize(i int) int {
	return u.size[u.find(i)]
}

// omittedMarkerLen returns the byte length of "... (N lines omitted) ...\n".
func omittedMarkerLen(n int) int {
	return len(fmt.Sprintf("... (%d lines omitted) ...\n", n))
}

// estimateOutputBytes calculates the approximate output size in bytes.
func estimateOutputBytes(lines []string, keep []bool, total int) int {
	bytes := 0
	inOmitted := false
	omittedCount := 0

	for i := 0; i < total; i++ {
		if keep[i] {
			if inOmitted {
				marker := fmt.Sprintf("... (%d lines omitted) ...\n", omittedCount)
				bytes += len(marker)
				inOmitted = false
				omittedCount = 0
			}
			bytes += len(lines[i]) + 1 // +1 for newline
		} else {
			inOmitted = true
			omittedCount++
		}
	}

	if inOmitted {
		marker := fmt.Sprintf("... (%d lines omitted) ...\n", omittedCount)
		bytes += len(marker)
	}

	return bytes
}
