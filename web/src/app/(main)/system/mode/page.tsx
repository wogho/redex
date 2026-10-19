"use client";

import * as React from "react";

import { CpuIcon, RefreshCwIcon } from "lucide-react";

import { Badge } from "@/components/ui/badge";
import { Button } from "@/components/ui/button";
import { Card, CardContent, CardDescription, CardHeader, CardTitle } from "@/components/ui/card";
import { api } from "@/lib/api";

type Info = {
  version: string;
  mode: string;
  offensive: boolean;
  require_human: boolean;
  language: string;
  policy_path: string;
  policy_configured: boolean;
  policy_exists: boolean;
  attack_layer_path: string;
  attack_layer_ready: boolean;
};

export default function ModePage() {
  const [info, setInfo] = React.useState<Info | null>(null);
  const [loading, setLoading] = React.useState(true);

  const load = React.useCallback(async () => {
    setLoading(true);
    try {
      setInfo(await api.systemInfo());
    } finally {
      setLoading(false);
    }
  }, []);

  React.useEffect(() => {
    void load();
  }, [load]);

  const rows: Array<[string, React.ReactNode]> = info
    ? [
        ["버전", <span className="font-mono text-xs">{info.version}</span>],
        ["모드", <Badge variant={info.offensive ? "destructive" : "default"}>{info.mode}</Badge>],
        ["사람 승인 강제", <Badge variant="outline">{info.require_human ? "ON" : "OFF"}</Badge>],
        ["출력 언어", info.language],
        ["정책 파일", <span className="font-mono text-xs">{info.policy_path || "미설정"}</span>],
        ["정책 상태", <Badge variant={info.policy_exists ? "default" : "outline"}>{info.policy_exists ? "적용 가능" : "없음"}</Badge>],
        ["ATT&CK 레이어", <span className="font-mono text-xs">{info.attack_layer_path}</span>],
        ["레이어 상태", <Badge variant={info.attack_layer_ready ? "default" : "outline"}>{info.attack_layer_ready ? "준비됨" : "없음"}</Badge>],
      ]
    : [];

  return (
    <div className="flex flex-col gap-6 p-6">
      <div className="flex flex-wrap items-center justify-between gap-3">
        <div>
          <h1 className="flex items-center gap-2 text-2xl font-semibold">
            <CpuIcon className="size-6 text-primary" />
            모드 · 런타임
          </h1>
          <p className="mt-1 text-sm text-muted-foreground">
            현재 프로세스의 실행 모드, 정책, ATT&CK 레이어 상태입니다. 변경은 compose env 로 하고 재기동합니다.
          </p>
        </div>
        <Button variant="outline" size="sm" onClick={() => void load()} disabled={loading}>
          <RefreshCwIcon className={loading ? "size-4 animate-spin" : "size-4"} />
          새로고침
        </Button>
      </div>

      <Card>
        <CardHeader className="pb-2">
          <CardTitle className="text-base">런타임 정보</CardTitle>
          <CardDescription>읽기 전용</CardDescription>
        </CardHeader>
        <CardContent className="divide-y">
          {rows.map(([k, v]) => (
            <div key={k} className="flex items-center justify-between gap-4 py-2 text-sm">
              <span className="text-muted-foreground">{k}</span>
              <span className="text-right">{v}</span>
            </div>
          ))}
        </CardContent>
      </Card>

      <Card>
        <CardHeader className="pb-2">
          <CardTitle className="text-base">저장소</CardTitle>
          <CardDescription>무거운 산출물은 OneDrive 로 오프로드됩니다 (`deploy/offload-onedrive.sh`).</CardDescription>
        </CardHeader>
        <CardContent className="text-sm text-muted-foreground">
          현재 이미지·소스·DB 볼륨만 로컬에 유지하고, 이미지 백업/소스 번들/DB 덤프는 <code className="font-mono">onedrive:redex</code> 에 보관합니다.
        </CardContent>
      </Card>
    </div>
  );
}
