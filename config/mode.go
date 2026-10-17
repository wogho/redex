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

// Offensive reports whether the process runs in offensive mode.
//
// 이 값은 프로세스 환경에서 매 호출 읽는다. 에이전트 시스템 프롬프트는 매 턴/매
// 라운드 다시 렌더되므로, 재기동 없이 바꾸는 것은 지원하지 않는다(일관성 우선).
func Offensive() bool {
	return strings.EqualFold(strings.TrimSpace(os.Getenv(ModeEnv)), "offensive")
}
