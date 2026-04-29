package cmd

import "testing"

func TestNormalizeOverviewDepth(t *testing.T) {
	tests := []struct {
		name    string
		depth   int
		want    int
		wantErr bool
	}{
		{name: "minimum", depth: 1, want: 1},
		{name: "default", depth: 2, want: 2},
		{name: "maximum", depth: 3, want: 3},
		{name: "too low", depth: 0, wantErr: true},
		{name: "too high", depth: 4, wantErr: true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := normalizeOverviewDepth(tt.depth)
			if tt.wantErr {
				if err == nil {
					t.Fatalf("expected error")
				}
				return
			}
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if got != tt.want {
				t.Fatalf("got %d, want %d", got, tt.want)
			}
		})
	}
}

func TestNormalizePositiveLimit(t *testing.T) {
	if got := normalizePositiveLimit(4, 8); got != 4 {
		t.Fatalf("positive limit got %d, want 4", got)
	}
	if got := normalizePositiveLimit(0, 8); got != 8 {
		t.Fatalf("zero limit got %d, want 8", got)
	}
	if got := normalizePositiveLimit(-1, 8); got != 8 {
		t.Fatalf("negative limit got %d, want 8", got)
	}
}
