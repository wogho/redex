package verify

import (
	"strings"
	"testing"
)

func TestRejectsThinEvidence(t *testing.T) {
	r := Analyze(Input{VulnClass: "sqli", Name: "SQLi", Severity: "high", Evidence: "looks vulnerable"})
	if r.Status != StatusRejected {
		t.Fatalf("status = %q, want rejected", r.Status)
	}
	if r.Confidence != 0 {
		t.Fatalf("confidence = %d, want 0", r.Confidence)
	}
}

func TestEvidenceBacked(t *testing.T) {
	ev := "$ curl -s 'http://target/item?id=1%20OR%201=1' -H 'Cookie: sess=abc'\n" +
		"HTTP/1.1 200 OK\nContent-Type: text/html\n\n" +
		"<html>admin panel: 12 users, emails dumped</html>\n" +
		"uid=33(www-data) gid=33(www-data)\n"
	r := Analyze(Input{
		VulnClass: "SQL Injection", Name: "Boolean-based SQLi in /item?id",
		Severity: "critical", Summary: "id 파라미터에서 boolean 기반 SQL 주입 확인, admin 데이터 열람 가능",
		Evidence: ev, AssetIDs: []int64{42, 7}, TrafficRefs: 2,
	})
	if r.Status != StatusEvidenceBacked {
		t.Fatalf("status = %q, want evidence-backed (confidence=%d reasons=%v)", r.Status, r.Confidence, r.Reasons)
	}
	if r.Confidence < 60 {
		t.Fatalf("confidence = %d, want >= 60", r.Confidence)
	}
}

func TestUnverifiedWithoutCommand(t *testing.T) {
	ev := strings.Repeat("서버 응답이 비정상적으로 보였고 관리자 페이지로 추정되는 화면이 있었다. ", 4)
	r := Analyze(Input{VulnClass: "auth", Name: "Broken auth", Severity: "medium", Evidence: ev})
	if r.Status != StatusUnverified {
		t.Fatalf("status = %q, want unverified (confidence=%d)", r.Status, r.Confidence)
	}
}

func TestDedupeKeyStable(t *testing.T) {
	a := DedupeKey("SQL Injection", "SQLi in /login", []int64{7, 42})
	b := DedupeKey("sql injection!", "  sqli   in /login ", []int64{42, 7})
	if a != b {
		t.Fatalf("dedupe keys differ: %s vs %s", a, b)
	}
	c := DedupeKey("SQL Injection", "SQLi in /login", []int64{7})
	if a == c {
		t.Fatal("dedupe key must include assets")
	}
}
