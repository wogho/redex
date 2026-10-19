package guard

import (
	"os"
	"path/filepath"
	"testing"
)

func writeTempPolicy(t *testing.T, body string) string {
	t.Helper()
	dir := t.TempDir()
	path := filepath.Join(dir, "policy.json")
	if err := os.WriteFile(path, []byte(body), 0o600); err != nil {
		t.Fatal(err)
	}
	return path
}

func TestPolicyDeny(t *testing.T) {
	if err := LoadPolicy(writeTempPolicy(t, `{"deny":["rm\\s+-rf\\s+/(\\s|$)","mkfs\\."],"deny_tools":["shell_open"]}`)); err != nil {
		t.Fatal(err)
	}
	p := activePolicy()
	if blocked, _ := p.Check("Bash", "rm -rf / --no-preserve-root"); !blocked {
		t.Fatal("expected destructive rm to be blocked")
	}
	if blocked, _ := p.Check("Bash", "rm -rf /home/ubuntu/tmpdir"); blocked {
		t.Fatal("scoped rm must be allowed")
	}
	if blocked, _ := p.Check("shell_open", ""); !blocked {
		t.Fatal("denied tool must be blocked")
	}
	t.Cleanup(func() { _ = LoadPolicy("") })
}

func TestPolicyAllowList(t *testing.T) {
	if err := LoadPolicy(writeTempPolicy(t, `{"allow":["^(Bash|http_request)\\b"]}`)); err != nil {
		t.Fatal(err)
	}
	p := activePolicy()
	if blocked, _ := p.Check("Bash", "nmap -sV target"); blocked {
		t.Fatal("Bash should match allow list")
	}
	if blocked, _ := p.Check("write_file", "/etc/passwd"); !blocked {
		t.Fatal("tool outside allow list must be blocked")
	}
	t.Cleanup(func() { _ = LoadPolicy("") })
}
