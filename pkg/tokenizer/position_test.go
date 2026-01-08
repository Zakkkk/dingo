package tokenizer

import (
	"testing"
)

func TestTokenizerPositions(t *testing.T) {
	tests := []struct {
		name     string
		src      string
		expected []struct {
			lit     string
			bytePos int // Actual byte position in source (0-based)
			byteEnd int // Actual byte end position in source (0-based, exclusive)
		}
	}{
		{
			name: "match expression",
			src:  "match x { A => 1 }",
			expected: []struct {
				lit     string
				bytePos int
				byteEnd int
			}{
				{"match", 0, 5},
				{"x", 6, 7},
				{"{", 8, 9},
				{"A", 10, 11},
				{"=>", 12, 14},
				{"1", 15, 16},
				{"}", 17, 18},
			},
		},
		{
			name: "lambda expression",
			src:  "|x| x + 1",
			expected: []struct {
				lit     string
				bytePos int
				byteEnd int
			}{
				{"|", 0, 1},
				{"x", 1, 2},
				{"|", 2, 3},
				{"x", 4, 5},
				{"+", 6, 7},
				{"1", 8, 9},
			},
		},
		{
			name: "simple identifier",
			src:  "hello",
			expected: []struct {
				lit     string
				bytePos int
				byteEnd int
			}{
				{"hello", 0, 5},
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			tok := New([]byte(tt.src))
			tokens, err := tok.Tokenize()
			if err != nil {
				t.Fatalf("Tokenize() error = %v", err)
			}

			// Remove EOF token for comparison
			if len(tokens) > 0 && tokens[len(tokens)-1].Kind == EOF {
				tokens = tokens[:len(tokens)-1]
			}

			if len(tokens) != len(tt.expected) {
				t.Fatalf("got %d tokens, want %d", len(tokens), len(tt.expected))
			}

			for i, exp := range tt.expected {
				token := tokens[i]

				// Use helper methods to get 0-based byte offsets
				bytePos := token.BytePos()
				byteEnd := token.ByteEnd()

				// Verify we can extract the token text using byte positions
				if bytePos < 0 || byteEnd > len(tt.src) || bytePos > byteEnd {
					t.Errorf("token[%d] invalid positions: bytePos=%d, byteEnd=%d, src len=%d",
						i, bytePos, byteEnd, len(tt.src))
					continue
				}

				extracted := tt.src[bytePos:byteEnd]
				if extracted != exp.lit {
					t.Errorf("token[%d] src[%d:%d] = %q, want %q (token.Pos=%d, token.End=%d)",
						i, bytePos, byteEnd, extracted, exp.lit, token.Pos, token.End)
				}

				// Verify expected byte positions match
				if bytePos != exp.bytePos {
					t.Errorf("token[%d] bytePos = %d, want %d", i, bytePos, exp.bytePos)
				}
				if byteEnd != exp.byteEnd {
					t.Errorf("token[%d] byteEnd = %d, want %d", i, byteEnd, exp.byteEnd)
				}
			}
		})
	}
}
