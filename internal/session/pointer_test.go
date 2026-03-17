package session

import "testing"

func TestFormatAndParseOTPointerRoundTrip(t *testing.T) {
	t.Parallel()

	value := FormatOTPointer([]int64{3, 1, 3, 2})
	pointer, err := ParseOTPointer(value)
	if err != nil {
		t.Fatalf("parse ot pointer: %v", err)
	}
	if len(pointer.Lines) != 3 || pointer.Lines[0] != 1 || pointer.Lines[1] != 2 || pointer.Lines[2] != 3 {
		t.Fatalf("unexpected lines: %+v", pointer.Lines)
	}
}

func TestParseOTPointerRejectsInvalidValue(t *testing.T) {
	t.Parallel()

	if _, err := ParseOTPointer("not-a-pointer"); err == nil {
		t.Fatal("expected invalid pointer to fail")
	}
}

func TestParseOTPointerRejectsCrossSessionValue(t *testing.T) {
	t.Parallel()

	if _, err := ParseOTPointer("ot-pointer://current/S1?lines=1"); err == nil {
		t.Fatal("expected cross-session pointer to fail")
	}
}
