package guard

import (
	"encoding/json"
	"fmt"
	"os"
	"regexp"
	"strings"
	"sync"
	"sync/atomic"

	"github.com/wogho/redex/config"
)

// Policy is the operator-supplied command policy. It is the safety floor that
// survives offensive mode: the gate bypass removes approvals, not the right of
// the operator to declare "never run this".
//
// File shape (REDEX_POLICY_FILE):
//
//	{
//	  "deny":       ["rm -rf /(?!home|tmp)", "mkfs\\."],
//	  "allow":      [],
//	  "deny_tools": ["shell_open"]
//	}
//
// deny       regexes matched against "<tool>\n<command>"
// allow      when non-empty, only matching calls are permitted (default deny)
// deny_tools exact tool-name denylist
type Policy struct {
	Deny      []string `json:"deny"`
	Allow     []string `json:"allow"`
	DenyTools []string `json:"deny_tools"`
}

type compiledPolicy struct {
	deny      []*regexp.Regexp
	allow     []*regexp.Regexp
	denyTools map[string]bool
	path      string
}

var (
	loadedPolicy atomic.Pointer[compiledPolicy]
	loadOnce     sync.Once
	loadErr      error
)

// LoadPolicy compiles a policy file and installs it. An empty path installs an
// empty policy (explicitly disabling operator rules).
func LoadPolicy(path string) error {
	var p Policy
	if path != "" {
		raw, err := os.ReadFile(path)
		if err != nil {
			return fmt.Errorf("policy: %w", err)
		}
		if err := json.Unmarshal(raw, &p); err != nil {
			return fmt.Errorf("policy parse %s: %w", path, err)
		}
	}
	cp := &compiledPolicy{path: path, denyTools: map[string]bool{}}
	for _, expr := range p.Deny {
		re, err := regexp.Compile(expr)
		if err != nil {
			return fmt.Errorf("policy deny %q: %w", expr, err)
		}
		cp.deny = append(cp.deny, re)
	}
	for _, expr := range p.Allow {
		re, err := regexp.Compile(expr)
		if err != nil {
			return fmt.Errorf("policy allow %q: %w", expr, err)
		}
		cp.allow = append(cp.allow, re)
	}
	for _, t := range p.DenyTools {
		cp.denyTools[strings.TrimSpace(t)] = true
	}
	loadedPolicy.Store(cp)
	loadErr = nil
	return nil
}

// activePolicy returns the installed policy, lazily loading REDEX_POLICY_FILE
// on first use.
func activePolicy() *compiledPolicy {
	loadOnce.Do(func() {
		if loadedPolicy.Load() != nil {
			return
		}
		path := strings.TrimSpace(os.Getenv(config.PolicyFileEnv))
		if path == "" {
			loadedPolicy.Store(&compiledPolicy{})
			return
		}
		if err := LoadPolicy(path); err != nil {
			loadErr = err
			loadedPolicy.Store(&compiledPolicy{})
			fmt.Fprintf(os.Stderr, "[policy] %v (continuing with empty policy)\n", err)
		}
	})
	return loadedPolicy.Load()
}

// PolicyError exposes the load failure (nil when the policy loaded cleanly).
func PolicyError() error { activePolicy(); return loadErr }

// Check reports whether a tool call is forbidden by policy.
func (p *compiledPolicy) Check(tool, cmd string) (bool, string) {
	if p == nil {
		return false, ""
	}
	if p.denyTools[tool] {
		return true, fmt.Sprintf("도구 %q 는 운영 정책에서 금지되어 있습니다", tool)
	}
	target := tool + "\n" + cmd
	for _, re := range p.deny {
		if re.MatchString(target) {
			return true, fmt.Sprintf("운영 정책 금지 패턴(%s)과 일치합니다", re.String())
		}
	}
	if len(p.allow) > 0 {
		for _, re := range p.allow {
			if re.MatchString(target) {
				return false, ""
			}
		}
		return true, "운영 정책 허용 목록에 없는 호출입니다"
	}
	return false, ""
}

// checkToolPolicy is the package-level check used by the Guard hook.
func checkToolPolicy(tool, cmd string) (bool, string) {
	return activePolicy().Check(tool, cmd)
}
