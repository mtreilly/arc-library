// Copyright (c) 2025 Arc Engineering
// SPDX-License-Identifier: MIT

package cmd

// truncate shortens a string to maxLen characters, adding "..." if truncated.
func truncate(s string, maxLen int) string {
	if len(s) <= maxLen {
		return s
	}
	return s[:maxLen-3] + "..."
}
