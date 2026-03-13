package tui

import "testing"

func TestParseDashboardCommand(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name  string
		input string
		kind  string
	}{
		{name: "exit", input: "/exit", kind: commandExit},
		{name: "prompt", input: "hi", kind: commandNone},
	}

	for _, test := range tests {
		test := test
		t.Run(test.name, func(t *testing.T) {
			t.Parallel()

			got := parseDashboardCommand(test.input)
			if got.kind != test.kind {
				t.Fatalf("unexpected command parse result: %+v", got)
			}
		})
	}
}
