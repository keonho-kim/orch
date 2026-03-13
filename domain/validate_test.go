package domain

import "testing"

func TestClipTask(t *testing.T) {
	t.Parallel()

	got := ClipTask("a very long sentence for clipping", 10)
	if got != "a very ..." {
		t.Fatalf("unexpected clipped value: %q", got)
	}
}
