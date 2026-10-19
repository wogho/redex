"use client";

import { RoadmapPage } from "@/app/(main)/_components/roadmap-page";

export default function OsintPage() {
  return (
    <RoadmapPage
      title="OSINT / 정찰 보드"
      desc="닉네임·이메일·전화·사진으로 후보 신원을 수집하고 신뢰도로 정렬합니다."
      bullets={[
        "닉네임 → Maigret(3,000+ 사이트) / Sherlock / WhatsMyName 계정 탐지",
        "이메일 → Holehe / GHunt / HIBP 브리치 조회",
        "전화 → PhoneInfoga, 사진 → PimEyes/FaceCheck API(유료) + Yandex 폴백",
        "후보 테이블 + 검증 게이트 재사용(증거 없는 후보는 unverified)",
        "교전 범위(RoE)에 '인물 조사 허용'이 있을 때만 실행",
      ]}
      note="법적 제약: 학적/유출 DB 조회, 비공개 계정 크롤링은 제외. 공개 정보 + 교전 범위 내에서만 동작합니다."
    />
  );
}
