package diff

import (
	"github.com/aryann/difflib"
)

// diffLines computes a line-level diff between two sequences using Hirschberg's
// linear-space LCS algorithm, producing output compatible with difflib.Diff.
//
// Unlike the O(N*M) space used by difflib.Diff's full-matrix dynamic
// programming approach, this implementation requires only O(N+M) space,
// which prevents excessive memory consumption when diffing large manifests.
//
// See https://github.com/databus23/helm-diff/issues/996
func diffLines(seq1, seq2 []string) []difflib.DiffRecord {
	start, end := numEqualStartAndEndElements(seq1, seq2)

	var result []difflib.DiffRecord

	for i := 0; i < start; i++ {
		result = append(result, difflib.DiffRecord{Payload: seq1[i], Delta: difflib.Common})
	}

	mid1 := seq1[start : len(seq1)-end]
	mid2 := seq2[start : len(seq2)-end]
	result = append(result, hirschbergDiff(mid1, mid2)...)

	for i := len(seq1) - end; i < len(seq1); i++ {
		result = append(result, difflib.DiffRecord{Payload: seq1[i], Delta: difflib.Common})
	}

	return result
}

func numEqualStartAndEndElements(seq1, seq2 []string) (start, end int) {
	for start < len(seq1) && start < len(seq2) && seq1[start] == seq2[start] {
		start++
	}
	i, j := len(seq1)-1, len(seq2)-1
	for i > start && j > start && seq1[i] == seq2[j] {
		i--
		j--
		end++
	}
	return
}

// hirschbergDiff recursively computes the LCS-based diff in linear space.
//
// The algorithm splits seq1 in half, finds the optimal split point in seq2
// using forward and backward LCS score rows, then recurses on each half.
// At each recursion level only O(len(seq2)) extra space is used.
func hirschbergDiff(seq1, seq2 []string) []difflib.DiffRecord {
	n, m := len(seq1), len(seq2)

	if n == 0 {
		result := make([]difflib.DiffRecord, m)
		for i, s := range seq2 {
			result[i] = difflib.DiffRecord{Payload: s, Delta: difflib.RightOnly}
		}
		return result
	}
	if m == 0 {
		result := make([]difflib.DiffRecord, n)
		for i, s := range seq1 {
			result[i] = difflib.DiffRecord{Payload: s, Delta: difflib.LeftOnly}
		}
		return result
	}
	if n == 1 {
		return singleRowDiff(seq1[0], seq2)
	}

	mid := n / 2

	forward := lcsLift(seq1[:mid], seq2)
	backward := lcsLiftSuffix(seq1[mid:], seq2)

	splitJ := 0
	maxScore := -1
	for j := 0; j <= m; j++ {
		score := forward[j] + backward[j]
		if score > maxScore {
			maxScore = score
			splitJ = j
		}
	}

	left := hirschbergDiff(seq1[:mid], seq2[:splitJ])
	right := hirschbergDiff(seq1[mid:], seq2[splitJ:])

	return append(left, right...)
}

// singleRowDiff handles the base case where seq1 has exactly one element.
// It matches difflib.Diff's behavior which prefers LeftOnly and matches
// at the latest possible position in seq2.
func singleRowDiff(elem string, seq2 []string) []difflib.DiffRecord {
	result := make([]difflib.DiffRecord, 0, len(seq2)+1)

	found := -1
	for j := len(seq2) - 1; j >= 0; j-- {
		if elem == seq2[j] {
			found = j
			break
		}
	}

	if found >= 0 {
		for j := 0; j < found; j++ {
			result = append(result, difflib.DiffRecord{Payload: seq2[j], Delta: difflib.RightOnly})
		}
		result = append(result, difflib.DiffRecord{Payload: elem, Delta: difflib.Common})
		for j := found + 1; j < len(seq2); j++ {
			result = append(result, difflib.DiffRecord{Payload: seq2[j], Delta: difflib.RightOnly})
		}
	} else {
		result = append(result, difflib.DiffRecord{Payload: elem, Delta: difflib.LeftOnly})
		for _, s := range seq2 {
			result = append(result, difflib.DiffRecord{Payload: s, Delta: difflib.RightOnly})
		}
	}

	return result
}

// lcsLift computes the last row of the standard LCS DP matrix.
// result[j] = LCS(a, b[:j]) for j = 0..len(b). Uses O(len(b)) space.
func lcsLift(a, b []string) []int {
	prev := make([]int, len(b)+1)
	curr := make([]int, len(b)+1)

	for i := 0; i < len(a); i++ {
		ai := a[i]
		for j := 0; j < len(b); j++ {
			if ai == b[j] {
				curr[j+1] = prev[j] + 1
			} else if prev[j+1] >= curr[j] {
				curr[j+1] = prev[j+1]
			} else {
				curr[j+1] = curr[j]
			}
		}
		prev, curr = curr, prev
		clearRow(curr)
	}

	return prev
}

// lcsLiftSuffix computes LCS(a, b[j:]) for each j = 0..len(b).
// It reverses both sequences, runs the standard forward DP, then
// maps the result back. Uses O(len(b)) space.
func lcsLiftSuffix(a, b []string) []int {
	ra := reverseStrings(a)
	rb := reverseStrings(b)

	// row[j] = LCS(ra, rb[:j]) = LCS(a, b[len(b)-j:])
	row := lcsLift(ra, rb)

	// backward[j] = LCS(a, b[j:]) = row[len(b)-j]
	result := make([]int, len(b)+1)
	for j := 0; j <= len(b); j++ {
		result[j] = row[len(b)-j]
	}
	return result
}

func reverseStrings(s []string) []string {
	result := make([]string, len(s))
	for i, v := range s {
		result[len(s)-1-i] = v
	}
	return result
}

func clearRow(row []int) {
	for i := range row {
		row[i] = 0
	}
}
