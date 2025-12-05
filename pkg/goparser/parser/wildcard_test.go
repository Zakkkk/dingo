package parser

import (
	"strings"
	"testing"
)

func TestWildcardBindings(t *testing.T) {
	tests := []struct {
		name     string
		input    string
		wantCode []string // code fragments that should be present
		wantNot  []string // code fragments that should NOT be present
	}{
		{
			name: "single wildcard skips extraction",
			input: `match event {
				PaymentFailed(_, reason) => fmt.Sprintf("Error: %s", reason)
			}`,
			wantCode: []string{
				"reason := __matchVal.reason",
			},
			wantNot: []string{
				"_ := __matchVal._", // wildcard should not be extracted
				"_ = reason",         // no automatic suppression
			},
		},
		{
			name: "multiple wildcards",
			input: `match event {
				OrderPlaced(_, amount, _) => fmt.Sprintf("$%.2f", amount)
			}`,
			wantCode: []string{
				"amount := __matchVal.amount",
			},
			wantNot: []string{
				"_ := __matchVal._",
				"_ = amount",
			},
		},
		{
			name: "all wildcards",
			input: `match event {
				PaymentFailed(_, _) => 1
			}`,
			wantCode: []string{
				"case", // should still generate case block
			},
			wantNot: []string{
				":= __matchVal.", // no field extraction
				"_ = ",           // no suppression
			},
		},
		{
			name: "no wildcards - normal extraction",
			input: `match event {
				PaymentFailed(orderID, reason) => fmt.Sprintf("%s: %s", orderID, reason)
			}`,
			wantCode: []string{
				"orderID := __matchVal.orderID",
				"reason := __matchVal.reason",
			},
			wantNot: []string{
				"_ = orderID", // no automatic suppression anymore
				"_ = reason",
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			output := transformMatch([]byte(tt.input))
			outputStr := string(output)

			// Check for expected code fragments
			for _, want := range tt.wantCode {
				if !strings.Contains(outputStr, want) {
					t.Errorf("Expected output to contain %q\nGot:\n%s", want, outputStr)
				}
			}

			// Check for unexpected code fragments
			for _, notWant := range tt.wantNot {
				if strings.Contains(outputStr, notWant) {
					t.Errorf("Expected output NOT to contain %q\nGot:\n%s", notWant, outputStr)
				}
			}
		})
	}
}
