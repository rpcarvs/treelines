package cmd

import (
	"strings"
	"testing"
)

func TestOnboardTextMentionsCoreAgentFlow(t *testing.T) {
	required := []string{
		"treelines index",
		"treelines overview",
		"treelines module-graph",
		"treelines recap",
	}
	for _, item := range required {
		if !strings.Contains(onboardText, item) {
			t.Fatalf("onboard text missing %q", item)
		}
	}
}

func TestRecapTextMentionsCoreCommandGroups(t *testing.T) {
	required := []string{
		"Core agent flow",
		"Overview",
		"Setup and lifecycle",
		"Discovery",
		"Relationships",
		"SQL and automation",
	}
	for _, item := range required {
		if !strings.Contains(recapText, item) {
			t.Fatalf("recap text missing %q", item)
		}
	}
}
