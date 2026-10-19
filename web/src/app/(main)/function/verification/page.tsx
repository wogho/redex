"use client";

import * as React from "react";

import Link from "next/link";

import { ExternalLinkIcon, RefreshCwIcon, ShieldCheckIcon } from "lucide-react";
import { useTranslations } from "next-intl";

import { Badge } from "@/components/ui/badge";
import { Button } from "@/components/ui/button";
import { Card, CardContent, CardDescription, CardHeader, CardTitle } from "@/components/ui/card";
import { Checkbox } from "@/components/ui/checkbox";
import { Input } from "@/components/ui/input";
import { Select, SelectContent, SelectItem, SelectTrigger, SelectValue } from "@/components/ui/select";
import { EmptyState, ErrorState, LoadingState } from "@/components/states";
import { toast } from "sonner";

import { api } from "@/lib/api";

type VerificationRow = {
  finding_id: string;
  task_id?: number;
  task_description?: string;
  name: string;
  vulnclass: string;
  severity: string;
  status: string;
  confidence: number;
  verify_status: string;
  dedupe_key?: string;
  reasons?: string[];
  created_at: string;
};

const SEVERITY_VARIANT: Record<string, "destructive" | "default" | "secondary" | "outline"> = {
  critical: "destructive",
  high: "destructive",
  medium: "default",
  low: "secondary",
};

const STATUS_VARIANT: Record<string, "default" | "secondary" | "outline"> = {
  "evidence-backed": "default",
  unverified: "secondary",
  duplicate: "outline",
};

export default function VerificationPage() {
  const t = useTranslations("verification");
  const [rows, setRows] = React.useState<VerificationRow[]>([]);
  const [counts, setCounts] = React.useState<Record<string, number>>({});
  const [status, setStatus] = React.useState("all");
  const [minConfidence, setMinConfidence] = React.useState("0");
  const [loading, setLoading] = React.useState(true);
  const [error, setError] = React.useState("");
  const [sel, setSel] = React.useState<Set<string>>(new Set());
  const [clusters, setClusters] = React.useState<Array<{ dedupe_key: string; count: number; max_confidence: number; representative_id: string; representative_name: string }>>([]);

  const load = React.useCallback(async () => {
    setLoading(true);
    setError("");
    try {
      const q = new URLSearchParams();
      if (status !== "all") q.set("status", status);
      if (Number(minConfidence) > 0) q.set("min_confidence", minConfidence);
      q.set("limit", "500");
      const [list, summary] = await Promise.all([
        api.verificationFindings(q),
        api.verificationSummary(),
      ]);
      setRows(list.findings ?? []);
      setCounts(summary.counts ?? {});
      const cl = await api.verificationClusters();
      setClusters(cl.clusters ?? []);
    } catch (err) {
      setError(err instanceof Error ? err.message : String(err));
    } finally {
      setLoading(false);
    }
  }, [status, minConfidence]);

  React.useEffect(() => {
    void load();
  }, [load]);

  const bulk = async (status: string) => {
    try {
      await api.verificationBulk([...sel], status);
      toast.success(`${sel.size}건 처리`);
      setSel(new Set());
      await load();
    } catch (e) {
      toast.error(e instanceof Error ? e.message : String(e));
    }
  };

  return (
    <div className="flex flex-col gap-6 p-6">
      <div className="flex flex-wrap items-center justify-between gap-3">
        <div>
          <h1 className="flex items-center gap-2 text-2xl font-semibold">
            <ShieldCheckIcon className="size-6 text-primary" />
            {t("title")}
          </h1>
          <p className="mt-1 text-sm text-muted-foreground">{t("desc")}</p>
        </div>
        <Button variant="outline" size="sm" onClick={() => void load()} disabled={loading}>
          <RefreshCwIcon className={loading ? "size-4 animate-spin" : "size-4"} />
          {t("refresh")}
        </Button>
      </div>

      <div className="grid gap-3 sm:grid-cols-3">
        <Card>
          <CardHeader className="pb-2">
            <CardDescription>{t("evidence_backed")}</CardDescription>
            <CardTitle className="text-2xl">{counts["evidence-backed"] ?? 0}</CardTitle>
          </CardHeader>
        </Card>
        <Card>
          <CardHeader className="pb-2">
            <CardDescription>{t("unverified")}</CardDescription>
            <CardTitle className="text-2xl">{counts.unverified ?? 0}</CardTitle>
          </CardHeader>
        </Card>
        <Card>
          <CardHeader className="pb-2">
            <CardDescription>{t("duplicate")}</CardDescription>
            <CardTitle className="text-2xl">{counts.duplicate ?? 0}</CardTitle>
          </CardHeader>
        </Card>
      </div>

      <div className="flex flex-wrap items-center gap-3">
        <Select value={status} onValueChange={setStatus}>
          <SelectTrigger className="w-52">
            <SelectValue placeholder={t("status")} />
          </SelectTrigger>
          <SelectContent>
            <SelectItem value="all">{t("status_all")}</SelectItem>
            <SelectItem value="evidence-backed">{t("evidence_backed")}</SelectItem>
            <SelectItem value="unverified">{t("unverified")}</SelectItem>
            <SelectItem value="duplicate">{t("duplicate")}</SelectItem>
          </SelectContent>
        </Select>
        <div className="flex items-center gap-2">
          <span className="text-sm text-muted-foreground">{t("min_confidence")}</span>
          <Input
            className="w-24"
            inputMode="numeric"
            value={minConfidence}
            onChange={(e) => setMinConfidence(e.target.value.replace(/[^0-9]/g, ""))}
          />
        </div>
      </div>

      {error ? <ErrorState message={error} onRetry={() => void load()} /> : null}
      {clusters.length ? (
        <Card>
          <CardHeader className="pb-2">
            <CardTitle className="text-base">클러스터 ({clusters.length})</CardTitle>
            <CardDescription>같은 dedupe_key로 묶인 중복입니다. 대표를 남기고 나머지를 중복 처리합니다.</CardDescription>
          </CardHeader>
          <CardContent className="flex flex-col gap-2 text-sm">
            {clusters.map((c) => (
              <div key={c.dedupe_key} className="flex flex-wrap items-center justify-between gap-2 border-b py-1 last:border-0">
                <div>
                  <span className="font-mono text-xs">{c.dedupe_key.slice(0, 10)}</span> · {c.count}건 · 대표: {c.representative_name || c.representative_id} (conf {c.max_confidence})
                </div>
                <Button size="sm" variant="outline" onClick={async () => {
                  try {
                    await api.verificationMerge(c.dedupe_key, c.representative_id);
                    toast.success("중복 처리 완료");
                    await load();
                  } catch (e) {
                    toast.error(e instanceof Error ? e.message : String(e));
                  }
                }}>대표만 남기기</Button>
              </div>
            ))}
          </CardContent>
        </Card>
      ) : null}
      {sel.size ? (
        <div className="flex flex-wrap items-center gap-2 rounded-md border bg-muted/40 px-3 py-2 text-sm">
          <span>{sel.size}개 선택</span>
          <Button size="sm" onClick={() => bulk("confirmed")}>확정</Button>
          <Button size="sm" variant="outline" onClick={() => bulk("false_positive")}>기각</Button>
          <Button size="sm" variant="ghost" onClick={() => setSel(new Set())}>선택 해제</Button>
        </div>
      ) : null}
      {loading && rows.length === 0 ? <LoadingState /> : null}
      {!loading && !error && rows.length === 0 ? (
        <EmptyState title="검증할 항목이 없습니다" description="게이트를 통과한 발견이 생기면 여기에 표시됩니다." />
      ) : null}
      <Card>
        <CardContent className="p-0">
          <div className="overflow-x-auto">
            <table className="w-full text-sm">
              <thead className="border-b bg-muted/40 text-left text-xs uppercase text-muted-foreground">
                <tr>
                  <th className="px-4 py-2">
                    <Checkbox
                      checked={rows.length > 0 && rows.every((r) => sel.has(r.finding_id))}
                      onCheckedChange={(v) => setSel(v === true ? new Set(rows.map((r) => r.finding_id)) : new Set())}
                    />
                  </th>
                  <th className="px-4 py-2">{t("name")}</th>
                  <th className="px-4 py-2">{t("task")}</th>
                  <th className="px-4 py-2">{t("severity")}</th>
                  <th className="px-4 py-2">{t("confidence")}</th>
                  <th className="px-4 py-2">{t("status")}</th>
                  <th className="px-4 py-2">{t("open")}</th>
                </tr>
              </thead>
              <tbody>
                {rows.map((row) => (
                  <tr key={row.finding_id} className="border-b last:border-0 hover:bg-muted/30">
                    <td className="px-4 py-3">
                      <Checkbox
                        checked={sel.has(row.finding_id)}
                        onCheckedChange={(v) => {
                          const next = new Set(sel);
                          if (v === true) next.add(row.finding_id); else next.delete(row.finding_id);
                          setSel(next);
                        }}
                      />
                    </td>
                    <td className="px-4 py-3">
                      <div className="font-medium">{row.name || row.vulnclass || "-"}</div>
                      <div className="text-xs text-muted-foreground">{row.vulnclass}</div>
                    </td>
                    <td className="max-w-[280px] px-4 py-3 text-xs text-muted-foreground">
                      {row.task_description || "-"}
                    </td>
                    <td className="px-4 py-3">
                      <Badge variant={SEVERITY_VARIANT[row.severity] ?? "outline"}>{row.severity || "-"}</Badge>
                    </td>
                    <td className="px-4 py-3 font-mono">{row.confidence}</td>
                    <td className="px-4 py-3">
                      <Badge variant={STATUS_VARIANT[row.verify_status] ?? "outline"}>
                        {row.verify_status || "-"}
                      </Badge>
                      {row.dedupe_key ? (
                        <div className="mt-1 font-mono text-[10px] text-muted-foreground">cluster {row.dedupe_key.slice(0, 8)}</div>
                      ) : null}
                      {row.reasons?.length ? (
                        <div className="mt-1 max-w-[240px] text-xs text-muted-foreground">{row.reasons[0]}</div>
                      ) : null}
                    </td>
                    <td className="px-4 py-3">
                      <Button asChild variant="ghost" size="sm">
                        <Link href={`/function/findings/detail?id=${row.finding_id}`}>
                          <ExternalLinkIcon className="size-4" />
                        </Link>
                      </Button>
                    </td>
                  </tr>
                ))}
                {!loading && rows.length === 0 ? (
                  <tr>
                    <td colSpan={7} className="px-4 py-10 text-center text-sm text-muted-foreground">
                      {t("empty")}
                    </td>
                  </tr>
                ) : null}
              </tbody>
            </table>
          </div>
        </CardContent>
      </Card>
    </div>
  );
}
