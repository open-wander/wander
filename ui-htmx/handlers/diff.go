package handlers

import (
	"strings"

	"github.com/open-wander/wander/ui-htmx/templates/partials/jobspartials"
)

// computeDiff computes a line-by-line diff between two text strings.
// It uses a simple longest common subsequence (LCS) based approach.
func computeDiff(oldText, newText string) []jobspartials.DiffLine {
	oldLines := splitLines(oldText)
	newLines := splitLines(newText)

	// Use Myers diff algorithm simplified - LCS-based approach
	return computeLCSDiff(oldLines, newLines)
}

// splitLines splits text into lines, preserving empty lines
func splitLines(text string) []string {
	if text == "" {
		return []string{}
	}
	// Normalize line endings
	text = strings.ReplaceAll(text, "\r\n", "\n")
	text = strings.ReplaceAll(text, "\r", "\n")
	return strings.Split(text, "\n")
}

// computeLCSDiff computes a diff using a simplified LCS approach
func computeLCSDiff(oldLines, newLines []string) []jobspartials.DiffLine {
	// Build LCS table
	m, n := len(oldLines), len(newLines)

	// dp[i][j] = length of LCS of oldLines[0..i-1] and newLines[0..j-1]
	dp := make([][]int, m+1)
	for i := range dp {
		dp[i] = make([]int, n+1)
	}

	for i := 1; i <= m; i++ {
		for j := 1; j <= n; j++ {
			if oldLines[i-1] == newLines[j-1] {
				dp[i][j] = dp[i-1][j-1] + 1
			} else {
				dp[i][j] = max(dp[i-1][j], dp[i][j-1])
			}
		}
	}

	// Backtrack to build the diff
	var result []jobspartials.DiffLine
	i, j := m, n
	oldLineNum, newLineNum := m, n

	// Build result in reverse order
	var reversed []jobspartials.DiffLine

	for i > 0 || j > 0 {
		if i > 0 && j > 0 && oldLines[i-1] == newLines[j-1] {
			// Lines are the same - unchanged
			reversed = append(reversed, jobspartials.DiffLine{
				Type:       "unchanged",
				Content:    oldLines[i-1],
				OldLineNum: i,
				NewLineNum: j,
			})
			i--
			j--
			oldLineNum--
			newLineNum--
		} else if j > 0 && (i == 0 || dp[i][j-1] >= dp[i-1][j]) {
			// Line was added in new version
			reversed = append(reversed, jobspartials.DiffLine{
				Type:       "added",
				Content:    newLines[j-1],
				OldLineNum: 0,
				NewLineNum: j,
			})
			j--
			newLineNum--
		} else if i > 0 {
			// Line was removed from old version
			reversed = append(reversed, jobspartials.DiffLine{
				Type:       "removed",
				Content:    oldLines[i-1],
				OldLineNum: i,
				NewLineNum: 0,
			})
			i--
			oldLineNum--
		}
	}

	// Reverse to get correct order
	for k := len(reversed) - 1; k >= 0; k-- {
		result = append(result, reversed[k])
	}

	return result
}

// countDiffStats counts the number of additions and deletions in a diff
func countDiffStats(lines []jobspartials.DiffLine) (added, removed int) {
	for _, line := range lines {
		switch line.Type {
		case "added":
			added++
		case "removed":
			removed++
		}
	}
	return added, removed
}

// max returns the larger of two integers
func max(a, b int) int {
	if a > b {
		return a
	}
	return b
}
