"use client";

import * as React from "react";

import { DownloadIcon, RefreshCwIcon, ShieldIcon } from "lucide-react";

import { Badge } from "@/components/ui/badge";
import { Button } from "@/components/ui/button";
import { Card, CardContent, CardDescription, CardHeader, CardTitle } from "@/components/ui/card";
import { api } from "@/lib/api";

type Tech = { techniqueID: string; tactic: string; comment?: string };
type Layer = { name: string; total: number; counts: Record<string, number>; techniques: Tech[] };

const TACTIC_KO: Record<string, string> = {
  reconnaissance: "정찰",
  "initial-access": "초기 침투",
  execution: "실행",
  persistence: "지속성",
  "privilege-escalation": "권한 상승",
  "defense-evasion": "방어 회피",
  "credential-access": "자격증명 접근",
  discovery: "탐색",
  "lateral-movement": "횡이동",
  collection: "수집",
  "command-and-control": "C2",
  exfiltration: "유출",
  impact: "영향",
};

export default function AttackPage() {
  const [layer, setLayer] = React.useState<Layer | null>(null);
  const [loading, setLoading] = React.useState(true);

  const load = React.useCallback(async () => {
    setLoading(true);
    try {
      setLayer(await api.attackLayer());
    } finally {
      setLoading(false);
    }
  }, []);

  React.useEffect(() => {
    void load();
  }, [load]);

  const download = async () => {
    const blob = new Blob([JSON.stringify(layer, null, 2)], { type: "application/json" });
    const url = URL.createObjectURL(blob);
    const a = document.createElement("a");
    a.href = url;
    a.download = "redex_offense_layer.json";
    a.click();
    URL.revokeObjectURL(url);
  };

  return (
    <div className="flex flex-col gap-6 p-6">
      <div className="flex flex-wrap items-center justify-between gap-3">
        <div>
          <h1 className="flex items-center gap-2 text-2xl font-semibold">
            <ShieldIcon className="size-6 text-primary" />
            ATT&CK 커버리지
          </h1>
          <p className="mt-1 text-sm text-muted-foreground">
            이미지에 탑재된 도구로 실행 가능한 MITRE ATT&CK Enterprise 기법 (오펜스 기준).
          </p>
        </div>
        <div className="flex gap-2">
          <Button variant="outline" size="sm" onClick={() => void load()} disabled={loading}>
            <RefreshCwIcon className={loading ? "size-4 animate-spin" : "size-4"} />
            새로고침
          </Button>
          <Button variant="outline" size="sm" onClick={() => void download()} disabled={!layer}>
            <DownloadIcon className="size-4" />
            Navigator JSON
          </Button>
        </div>
      </div>

      <div className="grid gap-3 sm:grid-cols-2 lg:grid-cols-4">
        {Object.entries(layer?.counts ?? {}).map(([tactic, n]) => (
          <Card key={tactic}>
            <CardHeader className="pb-2">
              <CardDescription>{TACTIC_KO[tactic] ?? tactic}</CardDescription>
              <CardTitle className="text-2xl">{n}</CardTitle>
            </CardHeader>
          </Card>
        ))}
      </div>

      <Card>
        <CardHeader className="pb-2">
          <CardTitle className="text-base">기법 ({layer?.total ?? 0})</CardTitle>
          <CardDescription>Navigator 에서 열려면 JSON 을 업로드하세요.</CardDescription>
        </CardHeader>
        <CardContent className="p-0">
          <div className="overflow-x-auto">
            <table className="w-full text-sm">
              <thead className="border-b bg-muted/40 text-left text-xs uppercase text-muted-foreground">
                <tr>
                  <th className="px-4 py-2">Technique</th>
                  <th className="px-4 py-2">전술</th>
                  <th className="px-4 py-2">근거 도구</th>
                </tr>
              </thead>
              <tbody>
                {(layer?.techniques ?? []).map((t) => (
                  <tr key={t.techniqueID} className="border-b last:border-0">
                    <td className="px-4 py-2 font-mono text-xs">{t.techniqueID}</td>
                    <td className="px-4 py-2">
                      <Badge variant="outline">{TACTIC_KO[t.tactic] ?? t.tactic}</Badge>
                    </td>
                    <td className="px-4 py-2 text-muted-foreground">{t.comment ?? "-"}</td>
                  </tr>
                ))}
              </tbody>
            </table>
          </div>
        </CardContent>
      </Card>
    </div>
  );
}
