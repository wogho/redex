"use client";

import { RoadmapPage } from "@/app/(main)/_components/roadmap-page";

export default function SchedulesPage() {
  return (
    <RoadmapPage
      title="스케줄 / 지속 테스트"
      desc="야간 재스캔과 staging 반복 테스트를 예약합니다."
      bullets={[
        "cron 표현식으로 태스크 반복 실행 (staging / nightly)",
        "이전 결과와 diff: 신규·해결·변경 finding 자동 분류",
        "실패/중단 시 재시도 정책과 알림 연동",
        "현재는 에이전트 트리거(/system/agents)로 일부 대체 가능",
      ]}
    />
  );
}
