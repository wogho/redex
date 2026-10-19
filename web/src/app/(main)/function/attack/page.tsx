"use client";

import * as React from "react";

import { DownloadIcon, RefreshCwIcon, SearchIcon, ShieldIcon } from "lucide-react";

import { Badge } from "@/components/ui/badge";
import { Button } from "@/components/ui/button";
import { Card, CardContent, CardDescription, CardHeader, CardTitle } from "@/components/ui/card";
import { Input } from "@/components/ui/input";
import { Tabs, TabsContent, TabsList, TabsTrigger } from "@/components/ui/tabs";
import { api } from "@/lib/api";

type Tech = { techniqueID: string; tactic: string; comment?: string };
type Layer = { name: string; total: number; counts: Record<string, number>; techniques: Tech[] };
type Usage = { techniques: Array<{ id: string; tactic: string; count: number; examples: string[] }>; findings: number };
type CveResult = {
  cve: string;
  exploits: Array<{ id: string; file: string; description: string; date: string; type: string; platform: string }>;
  nvd: { id: string; description: string; score: number; severity: string } | null;
};

const TACTIC_KO: Record<string, string> = {
  reconnaissance: "정찰", "initial-access": "초기 침투", execution: "실행", persistence: "지속성",
  "privilege-escalation": "권한 상승", "defense-evasion": "방어 회피", "credential-access": "자격증명",
  discovery: "탐색", "lateral-movement": "횡이동", collection: "수집",
  "command-and-control": "C2", exfiltration: "유출", impact: "영향",
};

export default function AttackPage() {
  const [layer, setLayer] = React.useState<Layer | null>(null);
  const [usage, setUsage] = React.useState<Usage | null>(null);
  const [cve, setCve] = React.useState("");
  const [cveResult, setCveResult] = React.useState<CveResult | null>(null);
  const [busy, setBusy] = React.useState(false);
  const [error, setError] = React.useState("");

  const load = React.useCallback(async () => {
    setBusy(true);
    setError("");
    try {
      const [l, u] = await Promise.all([api.attackLayer(), api.attackUsage()]);
      setLayer(l);
      setUsage(u);
    } catch (err) {
      setError(err instanceof Error ? err.message : String(err));
    } finally {
      setBusy(false);
    }
  }, []);

  React.useEffect(() => {
    void load();
  }, [load]);

  const searchCve = async () => {
    setError("");
    try {
      setCveResult(await api.cveMap(cve.trim()));
    } catch (err) {
      setError(err instanceof Error ? err.message : String(err));
    }
  };

  const download = () => {
    if (!layer) return;
    const blob = new Blob([JSON.stringify(layer, null, 2)], { type: "application/json" });
    const a = document.createElement("a");
    a.href = URL.createObjectURL(blob);
    a.download = "redex_offense_layer.json";
    a.click();
  };

  const usedMap = new Map((usage?.techniques ?? []).map((t) => [t.id, t.count]));
  const coveredIDs = new Set((layer?.techniques ?? []).map((t) => t.techniqueID));
  const usedCount = [...usedMap.keys()].filter((id) => coveredIDs.has(id)).length;
  const unused = (layer?.techniques ?? []).filter((t) => !usedMap.has(t.techniqueID));

  return (
    <div className="flex flex-col gap-6 p-6">
      <div className="flex flex-wrap items-center justify-between gap-3">
        <div>
          <h1 className="flex items-center gap-2 text-2xl font-semibold">
            <ShieldIcon className="size-6 text-primary" />
            ATT&CK
          </h1>
          <p className="mt-1 text-sm text-muted-foreground">커버리지 · 이번 교전 사용 현황 · CVE↔ExploitDB 매핑</p>
        </div>
        <Button variant="outline" size="sm" onClick={() => void load()} disabled={busy}>
          <RefreshCwIcon className={busy ? "size-4 animate-spin" : "size-4"} />
          새로고침
        </Button>
      </div>

      {error ? <p className="text-sm text-destructive">{error}</p> : null}

      <Card id="coverageExplain">
        <CardHeader className="pb-2">
          <CardTitle className="text-base">이 화면 읽는 법</CardTitle>
          <CardDescription>
            <span className="block">커버리지 = 지금 이미지의 도구로 <b>실행 가능한</b> 기법 목록입니다.</span>
            <span className="block">사용 현황 = 이번 교전의 발견을 기법으로 매핑한 <b>실제 사용</b> 결과입니다.</span>
            <span className="block">커버리지 탭에서 “이번 교전 사용” 열이 0이면 아직 시도하지 않은 기법입니다.</span>
          </CardDescription>
        </CardHeader>
      </Card>

      <Tabs defaultValue="usage" className="gap-4">
        <TabsList>
          <TabsTrigger value="usage">사용 현황</TabsTrigger>
          <TabsTrigger value="coverage">커버리지</TabsTrigger>
          <TabsTrigger value="cve">CVE 매핑</TabsTrigger>
        </TabsList>

        <TabsContent value="usage" className="flex flex-col gap-4">
          <Card>
            <CardHeader className="pb-2">
              <CardTitle className="text-base">이번 교전에서 사용된 기법</CardTitle>
              <CardDescription>
                발견 {usage?.findings ?? 0}건을 ATT&CK 기법으로 매핑한 결과입니다(휴리스틱).
              </CardDescription>
            </CardHeader>
            <CardContent className="p-0">
              <div className="overflow-x-auto">
                <table className="w-full text-sm">
                  <thead className="border-b bg-muted/40 text-left text-xs uppercase text-muted-foreground">
                    <tr>
                      <th className="px-4 py-2">Technique</th>
                      <th className="px-4 py-2">전술</th>
                      <th className="px-4 py-2">발견 수</th>
                      <th className="px-4 py-2">예시</th>
                    </tr>
                  </thead>
                  <tbody>
                    {(usage?.techniques ?? []).map((t) => (
                      <tr key={t.id} className="border-b last:border-0">
                        <td className="px-4 py-2 font-mono text-xs">{t.id}</td>
                        <td className="px-4 py-2"><Badge variant="outline">{TACTIC_KO[t.tactic] ?? t.tactic}</Badge></td>
                        <td className="px-4 py-2">{t.count}</td>
                        <td className="max-w-[420px] px-4 py-2 text-xs text-muted-foreground">{t.examples.join(" · ")}</td>
                      </tr>
                    ))}
                    {(usage?.techniques ?? []).length === 0 ? (
                      <tr>
                        <td colSpan={4} className="px-4 py-10 text-center text-sm text-muted-foreground">
                          매핑된 발견이 없습니다.
                        </td>
                      </tr>
                    ) : null}
                  </tbody>
                </table>
              </div>
            </CardContent>
          </Card>
        </TabsContent>

        <TabsContent value="coverage" className="flex flex-col gap-4">
          <div className="grid gap-3 sm:grid-cols-3">
            <Card><CardHeader className="pb-2"><CardDescription>실행 가능 기법</CardDescription><CardTitle className="text-2xl">{layer?.total ?? 0}</CardTitle></CardHeader></Card>
            <Card><CardHeader className="pb-2"><CardDescription>이번 교전 사용</CardDescription><CardTitle className="text-2xl">{usedCount}</CardTitle></CardHeader></Card>
            <Card><CardHeader className="pb-2"><CardDescription>미사용 후보</CardDescription><CardTitle className="text-2xl">{unused.length}</CardTitle></CardHeader></Card>
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
              <CardDescription>Navigator에서 열려면 JSON을 업로드하세요.</CardDescription>
            </CardHeader>
            <CardContent className="p-0">
              <div className="overflow-x-auto">
                <table className="w-full text-sm">
                  <thead className="border-b bg-muted/40 text-left text-xs uppercase text-muted-foreground">
                    <tr>
                      <th className="px-4 py-2">Technique</th>
                      <th className="px-4 py-2">전술</th>
                      <th className="px-4 py-2">도구/근거</th>
                      <th className="px-4 py-2">이번 교전</th>
                      <th className="px-4 py-2">상태</th>
                    </tr>
                  </thead>
                  <tbody>
                    {(layer?.techniques ?? []).map((t) => (
                      <tr key={t.techniqueID} className="border-b last:border-0">
                        <td className="px-4 py-2 font-mono text-xs">{t.techniqueID}</td>
                        <td className="px-4 py-2"><Badge variant="outline">{TACTIC_KO[t.tactic] ?? t.tactic}</Badge></td>
                        <td className="px-4 py-2 text-muted-foreground">{t.comment ?? "-"}</td>
                        <td className="px-4 py-2">{usedMap.get(t.techniqueID) ?? 0}</td>
                        <td className="px-4 py-2">
                          {usedMap.has(t.techniqueID)
                            ? <Badge variant="default">사용</Badge>
                            : <Badge variant="outline">미사용</Badge>}
                        </td>
                      </tr>
                    ))}
                  </tbody>
                </table>
              </div>
            </CardContent>
          </Card>
          <Button variant="outline" size="sm" onClick={download} disabled={!layer} className="w-fit">
            <DownloadIcon className="size-4" />
            Navigator JSON
          </Button>
        </TabsContent>

        <TabsContent value="cve" className="flex flex-col gap-4">
          <Card>
            <CardHeader className="pb-2">
              <CardTitle className="text-base">CVE ↔ ExploitDB</CardTitle>
              <CardDescription>ExploitDB 팩과 NVD 팩이 설치되어 있어야 결과가 나옵니다.</CardDescription>
            </CardHeader>
            <CardContent className="flex flex-wrap items-end gap-2">
              <Input className="w-64" placeholder="CVE-2026-12345" value={cve} onChange={(e) => setCve(e.target.value)} />
              <Button onClick={() => void searchCve()}>
                <SearchIcon className="size-4" />
                조회
              </Button>
            </CardContent>
          </Card>

          {cveResult ? (
            <Card>
              <CardHeader className="pb-2">
                <CardTitle className="text-base">{cveResult.cve}</CardTitle>
                <CardDescription>
                  {cveResult.nvd ? `${cveResult.nvd.id} · CVSS ${cveResult.nvd.score || "-"} ${cveResult.nvd.severity ?? ""}` : "NVD 정보 없음"}
                </CardDescription>
              </CardHeader>
              <CardContent className="flex flex-col gap-3 text-sm">
                {cveResult.nvd?.description ? <p className="text-muted-foreground">{cveResult.nvd.description}</p> : null}
                <div>
                  <div className="mb-1 text-xs text-muted-foreground">ExploitDB ({cveResult.exploits.length})</div>
                  {cveResult.exploits.map((e) => (
                    <div key={e.id} className="border-b py-1 last:border-0">
                      <span className="font-mono text-xs">EDB-{e.id}</span> · {e.description}
                      <span className="text-xs text-muted-foreground"> {e.platform}/{e.type} {e.date}</span>
                    </div>
                  ))}
                  {cveResult.exploits.length === 0 ? <p className="text-xs text-muted-foreground">매칭 없음</p> : null}
                </div>
              </CardContent>
            </Card>
          ) : null}
        </TabsContent>
      </Tabs>
    </div>
  );
}
