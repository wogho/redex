"use client";

import * as React from "react";

import Link from "next/link";

import { ExternalLinkIcon, RefreshCwIcon, ShieldCheckIcon } from "lucide-react";
import { useTranslations } from "next-intl";

import { Badge } from "@/components/ui/badge";
import { Button } from "@/components/ui/button";
import { Card, CardContent, CardDescription, CardHeader, CardTitle } from "@/components/ui/card";
import { Input } from "@/components/ui/input";
import { Select, SelectContent, SelectItem, SelectTrigger, SelectValue } from "@/components/ui/select";
import { EmptyState, ErrorState, LoadingState } from "@/components/states";
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
    } catch (err) {
      setError(err instanceof Error ? err.message : String(err));
    } finally {
      setLoading(false);
    }
  }, [status, minConfidence]);

  React.useEffect(() => {
    void load();
  }, [load]);

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
                    <td colSpan={6} className="px-4 py-10 text-center text-sm text-muted-foreground">
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
