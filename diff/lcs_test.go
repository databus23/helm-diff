package diff

import (
	"fmt"
	"math/rand"
	"testing"

	"github.com/aryann/difflib"
	"github.com/stretchr/testify/require"
)

// TestDiffLinesParityWithDifflib verifies that diffLines produces identical
// output to difflib.Diff across a variety of inputs, ensuring the
// linear-space implementation is a drop-in replacement.
func TestDiffLinesParityWithDifflib(t *testing.T) {
	cases := []struct {
		name string
		seq1 []string
		seq2 []string
	}{
		{
			name: "both empty",
			seq1: nil,
			seq2: nil,
		},
		{
			name: "first empty",
			seq1: nil,
			seq2: []string{"a", "b"},
		},
		{
			name: "second empty",
			seq1: []string{"a", "b"},
			seq2: nil,
		},
		{
			name: "identical",
			seq1: []string{"a", "b", "c"},
			seq2: []string{"a", "b", "c"},
		},
		{
			name: "completely different",
			seq1: []string{"a", "b", "c"},
			seq2: []string{"x", "y", "z"},
		},
		{
			name: "common prefix only",
			seq1: []string{"a", "b", "c"},
			seq2: []string{"a", "b", "d"},
		},
		{
			name: "common suffix only",
			seq1: []string{"a", "b", "c"},
			seq2: []string{"x", "b", "c"},
		},
		{
			name: "interleaved changes",
			seq1: []string{"line1", "line2", "line3", "line4", "line5"},
			seq2: []string{"line1-changed", "line2", "line3-changed", "line4", "line5-changed"},
		},
		{
			name: "insertions",
			seq1: []string{"a", "c"},
			seq2: []string{"a", "b", "c"},
		},
		{
			name: "deletions",
			seq1: []string{"a", "b", "c"},
			seq2: []string{"a", "c"},
		},
		{
			name: "repeated elements",
			seq1: []string{"a", "a", "a", "b"},
			seq2: []string{"a", "b", "a"},
		},
		{
			name: "single element match at end",
			seq1: []string{"x"},
			seq2: []string{"a", "b", "x"},
		},
		{
			name: "single element match at start",
			seq1: []string{"x"},
			seq2: []string{"x", "a", "b"},
		},
		{
			name: "single element no match",
			seq1: []string{"x"},
			seq2: []string{"a", "b"},
		},
		{
			name: "multiple matches of single element",
			seq1: []string{"x"},
			seq2: []string{"x", "a", "x", "b", "x"},
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			expected := difflib.Diff(tc.seq1, tc.seq2)
			actual := diffLines(tc.seq1, tc.seq2)
			require.Equal(t, expected, actual, "diffLines output differs from difflib.Diff")
		})
	}
}

// TestDiffLinesSemanticValidity verifies that diffLines produces semantically
// valid diffs on random inputs. When multiple valid LCS paths exist,
// diffLines may choose a different path than difflib.Diff, so we verify
// correctness properties rather than exact output match.
func TestDiffLinesSemanticValidity(t *testing.T) {
	rnd := rand.New(rand.NewSource(42))

	for iter := 0; iter < 1000; iter++ {
		seq1 := generateRandomSequence(rnd, 1+rnd.Intn(30), 1+rnd.Intn(8))
		seq2 := generateRandomSequence(rnd, 1+rnd.Intn(30), 1+rnd.Intn(8))

		records := diffLines(seq1, seq2)

		i1, i2 := 0, 0
		commonCount := 0
		for _, r := range records {
			switch r.Delta {
			case difflib.Common:
				require.Less(t, i1, len(seq1), "iter %d: Common references seq1[%d] out of range", iter, i1)
				require.Less(t, i2, len(seq2), "iter %d: Common references seq2[%d] out of range", iter, i2)
				require.Equal(t, seq1[i1], r.Payload, "iter %d: Common payload mismatch at seq1[%d]", iter, i1)
				require.Equal(t, seq2[i2], r.Payload, "iter %d: Common payload mismatch at seq2[%d]", iter, i2)
				i1++
				i2++
				commonCount++
			case difflib.LeftOnly:
				require.Less(t, i1, len(seq1), "iter %d: LeftOnly references seq1[%d] out of range", iter, i1)
				require.Equal(t, seq1[i1], r.Payload, "iter %d: LeftOnly payload mismatch at seq1[%d]", iter, i1)
				i1++
			case difflib.RightOnly:
				require.Less(t, i2, len(seq2), "iter %d: RightOnly references seq2[%d] out of range", iter, i2)
				require.Equal(t, seq2[i2], r.Payload, "iter %d: RightOnly payload mismatch at seq2[%d]", iter, i2)
				i2++
			}
		}

		require.Equal(t, len(seq1), i1, "iter %d: not all seq1 elements consumed", iter)
		require.Equal(t, len(seq2), i2, "iter %d: not all seq2 elements consumed", iter)

		expectedLCS := lcsLength(seq1, seq2)
		require.Equal(t, expectedLCS, commonCount,
			"iter %d: LCS length mismatch (expected %d, got %d)\nseq1=%v\nseq2=%v",
			iter, expectedLCS, commonCount, seq1, seq2)
	}
}

// TestDiffLinesLargeInput ensures the implementation handles large inputs
// without excessive memory usage.
func TestDiffLinesLargeInput(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping large input test in short mode")
	}

	N := 5000
	seq1 := make([]string, N)
	seq2 := make([]string, N)
	for i := 0; i < N; i++ {
		seq1[i] = fmt.Sprintf("line %d", i)
		if i%3 == 0 {
			seq2[i] = fmt.Sprintf("changed %d", i)
		} else {
			seq2[i] = seq1[i]
		}
	}

	records := diffLines(seq1, seq2)
	require.NotEmpty(t, records)
}

// lcsLength computes the LCS length using the standard O(N*M) DP.
// Used only in tests to verify correctness.
func lcsLength(seq1, seq2 []string) int {
	prev := make([]int, len(seq2)+1)
	curr := make([]int, len(seq2)+1)
	for i := 0; i < len(seq1); i++ {
		for j := 0; j < len(seq2); j++ {
			if seq1[i] == seq2[j] {
				curr[j+1] = prev[j] + 1
			} else if prev[j+1] >= curr[j] {
				curr[j+1] = prev[j+1]
			} else {
				curr[j+1] = curr[j]
			}
		}
		prev, curr = curr, prev
		for i := range curr {
			curr[i] = 0
		}
	}
	return prev[len(seq2)]
}

func generateRandomSequence(rnd *rand.Rand, length, alphabetSize int) []string {
	result := make([]string, length)
	for i := 0; i < length; i++ {
		result[i] = fmt.Sprintf("item-%d", rnd.Intn(alphabetSize))
	}
	return result
}
