package config

import (
	"os"
	"strings"
)

// ModeEnv selects the runtime operating mode.
//
//	REDEX_MODE=offensive → intercept 승인 게이트를 끄고, 모든 펜테스트 에이전트
//	프롬프트에 공격(익스플로잇) 우선 지시문을 코드로 고정 주입한다.
//
// 비어 있거나 다른 값이면 상류 기본 동작(정찰·검증 중심 + 승인 게이트)을 유지한다.
const ModeEnv = "REDEX_MODE"

// PolicyFileEnv points at the operator command policy (see guard.Policy).
// An empty value disables operator policy.
const PolicyFileEnv = "REDEX_POLICY_FILE"

// RequireHumanEnv, when truthy, forces the approval gate back on even in
// offensive mode. It is the "someone must look at this" switch.
const RequireHumanEnv = "REDEX_REQUIRE_HUMAN"

// AttackLayerEnv overrides the embedded ATT&CK offense layer path.
const AttackLayerEnv = "REDEX_ATTACK_LAYER"

// Offensive reports whether the process runs in offensive mode.
//
// 이 값은 프로세스 환경에서 매 호출 읽는다. 에이전트 시스템 프롬프트는 매 턴/매
// 라운드 다시 렌더되므로, 재기동 없이 바꾸는 것은 지원하지 않는다(일관성 우선).
func Offensive() bool {
	return strings.EqualFold(strings.TrimSpace(os.Getenv(ModeEnv)), "offensive")
}

// RequireHuman reports whether every gated tool call must be approved by a
// human, even when REDEX_MODE=offensive.
func RequireHuman() bool {
	switch strings.ToLower(strings.TrimSpace(os.Getenv(RequireHumanEnv))) {
	case "1", "true", "yes", "on":
		return true
	}
	return false
}
