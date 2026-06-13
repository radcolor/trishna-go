package youtube

import "testing"

func TestIsLiveStream(t *testing.T) {
	if !isLiveStream("Live Sky: COTL") {
		t.Fatal("expected live stream title to match")
	}
	if isLiveStream("Sky daily reset guide") {
		t.Fatal("expected non-live title to not match")
	}
}

func TestWatchButtonLabel(t *testing.T) {
	if watchButtonLabel(true) != "Watch Live" {
		t.Fatalf("live label = %q", watchButtonLabel(true))
	}
	if watchButtonLabel(false) != "Watch Video" {
		t.Fatalf("upload label = %q", watchButtonLabel(false))
	}
}

func TestTruncateDescriptionTwoLines(t *testing.T) {
	input := "First line\nSecond line\nThird line"
	got := truncateDescription(input)
	want := "First line\nSecond line..."
	if got != want {
		t.Fatalf("description = %q", got)
	}
}

func TestTruncateDescriptionShort(t *testing.T) {
	input := "Only one line"
	got := truncateDescription(input)
	if got != "Only one line" {
		t.Fatalf("description = %q", got)
	}
}

func TestTruncateDescriptionEmpty(t *testing.T) {
	if got := truncateDescription(""); got != "" {
		t.Fatalf("description = %q", got)
	}
}

func TestTruncateDescriptionSkipsBlankLines(t *testing.T) {
	input := "\n\nFirst line\n\nSecond line\nThird"
	got := truncateDescription(input)
	want := "First line\nSecond line..."
	if got != want {
		t.Fatalf("description = %q", got)
	}
}
