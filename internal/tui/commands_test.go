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
		{name: "clear", input: "/clear", kind: commandClear},
		{name: "compact", input: "/compact", kind: commandCompact},
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

func TestFilteredSlashCommands(t *testing.T) {
	t.Parallel()

	got := filteredSlashCommands("/")
	if len(got) != 3 {
		t.Fatalf("unexpected slash commands for root slash: %+v", got)
	}

	got = filteredSlashCommands("/c")
	if len(got) != 2 || got[0].value != "/clear" || got[1].value != "/compact" {
		t.Fatalf("unexpected slash commands for /c: %+v", got)
	}

	got = filteredSlashCommands("hello")
	if len(got) != 0 {
		t.Fatalf("expected no slash commands for plain text, got %+v", got)
	}
}
