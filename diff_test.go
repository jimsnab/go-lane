package lane

import (
	"testing"
)

func TestDiffArray(t *testing.T) {
	tests := []struct {
		name     string
		a        []any
		b        []any
		expected string
	}{
		{
			name:     "No changes",
			a:        []any{1, 2, 3, 4},
			b:        []any{1, 2, 3, 4},
			expected: "",
		},
		{
			name:     "Insertion",
			a:        []any{1, 2, 3, 4},
			b:        []any{1, 2, 5, 3, 4},
			expected: `[insert[2]: 5]`,
		},
		{
			name:     "Multiple Insertions",
			a:        []any{1, 2, 3, 4},
			b:        []any{1, 5, 2, 6, 3, 4},
			expected: `[insert[1]: 5][insert[3]: 6]`,
		},
		{
			name:     "Deletion",
			a:        []any{1, 2, 3, 4},
			b:        []any{1, 3, 4},
			expected: `[remove[1]: 2]`,
		},
		{
			name:     "Multiple Deletions",
			a:        []any{1, 2, 3, 4},
			b:        []any{2, 4},
			expected: `[remove[0]: 1][remove[2]: 3]`,
		},
		{
			name:     "Replacement",
			a:        []any{1, 2, 3, 4},
			b:        []any{1, 2, 5, 4},
			expected: `[replace[2]: [3->5]]`,
		},
		{
			name:     "Insertions and Deletions",
			a:        []any{1, 2, 3, 4},
			b:        []any{2, 5, 4, 6},
			expected: `[remove[0]: 1][replace[1]: [3->5]][append[3]: 6]`,
		},
		{
			name:     "Complex Changes",
			a:        []any{1, "two", 3.0, true},
			b:        []any{1, "two", 4.0, false, "new"},
			expected: `[replace[2]: [3.000000->4.000000]][replace[3]: [true->false]][append[4]: "new"]`,
		},
		{
			name:     "Many Inserts, Deletes, Replacements, Moves",
			a:        []any{1, 2, 3, 4, 5, 6, 7, 8, 9, 10},
			b:        []any{5, 1, 11, 2, 12, 13, 7, 3, 8, 4, 14, 6, 15, 16, 10},
			expected: `[insert[0]: 5][insert[2]: 11][replace[4]: [3->12]][replace[5]: [4->13]][replace[6]: [5->7]][replace[7]: [6->3]][remove[6]: 7][replace[9]: [9->4]][replace[10]: [10->14]][append[11]: 6][append[12]: 15][append[13]: 16][append[14]: 10]`,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := diffArray(tt.a, tt.b)
			if result != tt.expected {
				t.Errorf("diffArray() = %v, want %v", result, tt.expected)
			}
		})
	}
}
