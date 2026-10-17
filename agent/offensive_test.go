package agent

import (
	"strings"
	"testing"
)

// The offensive tail is code-owned: present exactly when REDEX_MODE=offensive,
// appended after the rendered body so a DB-edited prompt can't drop it.
func TestOffensiveDirectiveGated(t *testing.T) {
	t.Setenv("REDEX_MODE", "")
	if got := offensiveDirective(); got != "" {
		t.Fatalf("default mode must inject nothing, got %q", got)
	}

	t.Setenv("REDEX_MODE", "offensive")
	got := offensiveDirective()
	for _, want := range []string{"进攻模式", "攻击链优先", "利用优先", "绕过优先", "/opt/wordlists"} {
		if !strings.Contains(got, want) {
			t.Errorf("offensive directive missing %q", want)
		}
	}
}

func TestWorkerSystemCarriesOffensiveTail(t *testing.T) {
	t.Setenv("REDEX_MODE", "offensive")
	sys := workerSystem("", "", "/data", "/data/tasks/1/i2")
	if !strings.Contains(sys, "进攻模式") {
		t.Fatalf("worker system prompt missing offensive tail")
	}
	if !strings.Contains(sys, "中间产物输出规约") {
		t.Fatalf("worker system prompt lost its artifact spec")
	}
}

func TestPlannerSystemCarriesOffensiveTail(t *testing.T) {
	t.Setenv("REDEX_MODE", "offensive")
	if !strings.Contains(plannerSystem("goal", "/data", "/data/tasks/1"), "进攻模式") {
		t.Fatalf("planner system prompt missing offensive tail")
	}
}
