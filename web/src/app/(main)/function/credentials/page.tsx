"use client";

import { RoadmapPage } from "@/app/(main)/_components/roadmap-page";

export default function CredentialsPage() {
  return (
    <RoadmapPage
      title="자격증명 보관소"
      desc="교전 중 수집한 자격증명을 암호화해 보관하고 재사용합니다."
      bullets={[
        "AES-GCM 암호화(마스터 키는 .secrets 에 보관, DB 에 평문 없음)",
        "자격증명 → 자산/서비스 연결, 재사용 이력 추적",
        "마스킹 표시 + 복사/다운로드 감사 로그",
        "만료·회전 알림",
      ]}
      note="보안상 자동 재사용은 기본 OFF, 교전 단위로 명시 허용해야 합니다."
    />
  );
}
