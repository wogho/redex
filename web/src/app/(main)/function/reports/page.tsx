"use client";

import * as React from "react";

import { FileTextIcon, RefreshCwIcon } from "lucide-react";

import { Badge } from "@/components/ui/badge";
import { Button } from "@/components/ui/button";
import { Card, CardContent, CardDescription, CardHeader, CardTitle } from "@/components/ui/card";
import { EmptyState, ErrorState, LoadingState } from "@/components/states";
import { api } from "@/lib/api";

type Stats = {
  total?: number;
  by_severity?: Record<string, number>;
  by_status?: Record<string, number>;
  by_vulnclass?: Record<string, number>;
};

export default function ReportsPage() {
  const [stats, setStats] = React.useState<Stats>({});
  const [tasks, setTasks] = React.useState<Array<{ id: string; name?: string; description?: string; status?: string }>>([]);
  const [loading, setLoading] = React.useState(true);
  const [error, setError] = React.useState("");

  const load = React.useCallback(async () => {
    setLoading(true);
    setError("");
    try {
      const [s, t] = await Promise.all([api.findingStats(), api.tasks()]);
      setStats(s as unknown as Stats);
      setTasks(t.tasks as Array<{ id: string; name?: string; description?: string; status?: string }>);
    } catch (err) {
      setError(err instanceof Error ? err.message : String(err));
    } finally {
      setLoading(false);
    }
  }, []);

  React.useEffect(() => {
    void load();
  }, [load]);

  return (
    <div className="flex flex-col gap-6 p-6">
      <div className="flex flex-wrap items-center justify-between gap-3">
        <div>
          <h1 className="flex items-center gap-2 text-2xl font-semibold">
            <FileTextIcon className="size-6 text-primary" />
            보고서 센터
          </h1>
          <p className="mt-1 text-sm text-muted-foreground">
            교전별 취약점 통계와 내보내기 진입점입니다. 상세 내보내기(MD/CSV/JSON)는 취약점 화면에서 선택 가능합니다.
          </p>
        </div>
        <Button variant="outline" size="sm" onClick={() => void load()} disabled={loading}>
          <RefreshCwIcon className={loading ? "size-4 animate-spin" : "size-4"} />
          새로고침
        </Button>
      </div>

      <div className="grid gap-3 sm:grid-cols-4">
        <Card>
          <CardHeader className="pb-2">
            <CardDescription>전체 발견</CardDescription>
            <CardTitle className="text-2xl">{stats.total ?? 0}</CardTitle>
          </CardHeader>
        </Card>
        {(["critical", "high", "medium", "low"] as const).map((sev) => (
          <Card key={sev}>
            <CardHeader className="pb-2">
              <CardDescription>{sev}</CardDescription>
              <CardTitle className="text-2xl">{stats.by_severity?.[sev] ?? 0}</CardTitle>
            </CardHeader>
          </Card>
        ))}
      </div>

      {error ? <ErrorState message={error} onRetry={() => void load()} /> : null}
      {loading && tasks.length === 0 ? <LoadingState /> : null}
      {!loading && !error && tasks.length === 0 ? (
        <EmptyState title="교전이 없습니다" description="태스크를 만들면 여기에 보고서 진입점이 생깁니다." />
      ) : null}
      <Card>
        <CardHeader className="pb-2">
          <CardTitle className="text-base">교전({tasks.length})</CardTitle>
          <CardDescription>태스크 상세의 보고서 탭에서 내보낼 수 있습니다.</CardDescription>
        </CardHeader>
        <CardContent className="p-0">
          <div className="overflow-x-auto">
            <table className="w-full text-sm">
              <thead className="border-b bg-muted/40 text-left text-xs uppercase text-muted-foreground">
                <tr>
                  <th className="px-4 py-2">태스크</th>
                  <th className="px-4 py-2">상태</th>
                  <th className="px-4 py-2">이동</th>
                </tr>
              </thead>
              <tbody>
                {tasks.map((t) => (
                  <tr key={t.id} className="border-b last:border-0 hover:bg-muted/30">
                    <td className="px-4 py-2">
                      <div className="font-medium">{t.name || t.description || `#${t.id}`}</div>
                      <div className="text-xs text-muted-foreground">#{t.id}</div>
                    </td>
                    <td className="px-4 py-2">
                      <Badge variant="outline">{t.status ?? "-"}</Badge>
                    </td>
                    <td className="px-4 py-2">
                      <a className="text-primary underline-offset-4 hover:underline" href={`/function/tasks/detail?id=${t.id}&tab=report`}>
                        보고서 열기
                      </a>
                    </td>
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
