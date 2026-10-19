"use client";

import * as React from "react";

import { PlusIcon, RefreshCwIcon, Trash2Icon, FileCheckIcon } from "lucide-react";
import { toast } from "sonner";

import { Button } from "@/components/ui/button";
import { Card, CardContent, CardDescription, CardHeader, CardTitle } from "@/components/ui/card";
import { Input } from "@/components/ui/input";
import { Label } from "@/components/ui/label";
import { Textarea } from "@/components/ui/textarea";
import { api } from "@/lib/api";

type Scope = {
  id: string;
  name: string;
  targets: string;
  exclusions: string;
  contact: string;
  starts_at: string;
  ends_at: string;
  notes: string;
};

const EMPTY: Scope = { id: "", name: "", targets: "", exclusions: "", contact: "", starts_at: "", ends_at: "", notes: "" };

export default function ScopePage() {
  const [rows, setRows] = React.useState<Scope[]>([]);
  const [form, setForm] = React.useState<Scope>(EMPTY);
  const [loading, setLoading] = React.useState(true);

  const load = React.useCallback(async () => {
    setLoading(true);
    try {
      const r = await api.scopeList();
      setRows((r.scopes ?? []) as unknown as Scope[]);
    } finally {
      setLoading(false);
    }
  }, []);

  React.useEffect(() => {
    void load();
  }, [load]);

  const save = async () => {
    if (!form.name.trim()) {
      toast.error("교전 이름을 입력하세요");
      return;
    }
    try {
      await api.scopeCreate(form);
      setForm(EMPTY);
      toast.success("교전 범위를 저장했습니다");
      await load();
    } catch (err) {
      toast.error(err instanceof Error ? err.message : String(err));
    }
  };

  const remove = async (id: string) => {
    try {
      await api.scopeDelete(id);
      await load();
    } catch (err) {
      toast.error(err instanceof Error ? err.message : String(err));
    }
  };

  return (
    <div className="flex flex-col gap-6 p-6">
      <div className="flex flex-wrap items-center justify-between gap-3">
        <div>
          <h1 className="flex items-center gap-2 text-2xl font-semibold">
            <FileCheckIcon className="size-6 text-primary" />
            교전 범위 (RoE)
          </h1>
          <p className="mt-1 text-sm text-muted-foreground">
            허가 대상·제외 범위·기간·연락처를 기록합니다. 실행 차단은 운영 정책이 담당하고, 이 화면은 감사 가능한 승인 기록입니다.
          </p>
        </div>
        <Button variant="outline" size="sm" onClick={() => void load()} disabled={loading}>
          <RefreshCwIcon className={loading ? "size-4 animate-spin" : "size-4"} />
          새로고침
        </Button>
      </div>

      <Card>
        <CardHeader className="pb-3">
          <CardTitle className="text-base">새 교전</CardTitle>
          <CardDescription>한 줄에 하나씩 대상/제외 범위를 적습니다.</CardDescription>
        </CardHeader>
        <CardContent className="grid gap-3 md:grid-cols-2">
          <div className="flex flex-col gap-1">
            <Label>이름</Label>
            <Input value={form.name} onChange={(e) => setForm({ ...form, name: e.target.value })} placeholder="2026 Q4 은행권 교전" />
          </div>
          <div className="flex flex-col gap-1">
            <Label>연락처</Label>
            <Input value={form.contact} onChange={(e) => setForm({ ...form, contact: e.target.value })} placeholder="security@example.com" />
          </div>
          <div className="flex flex-col gap-1">
            <Label>대상 범위</Label>
            <Textarea value={form.targets} onChange={(e) => setForm({ ...form, targets: e.target.value })} placeholder={"10.0.0.0/24\napp.example.com"} />
          </div>
          <div className="flex flex-col gap-1">
            <Label>제외 범위</Label>
            <Textarea value={form.exclusions} onChange={(e) => setForm({ ...form, exclusions: e.target.value })} placeholder={"prod-db.example.com"} />
          </div>
          <div className="flex flex-col gap-1">
            <Label>시작</Label>
            <Input value={form.starts_at} onChange={(e) => setForm({ ...form, starts_at: e.target.value })} placeholder="2026-11-01 09:00 KST" />
          </div>
          <div className="flex flex-col gap-1">
            <Label>종료</Label>
            <Input value={form.ends_at} onChange={(e) => setForm({ ...form, ends_at: e.target.value })} placeholder="2026-11-05 18:00 KST" />
          </div>
          <div className="flex flex-col gap-1 md:col-span-2">
            <Label>메모</Label>
            <Textarea value={form.notes} onChange={(e) => setForm({ ...form, notes: e.target.value })} placeholder="승인 문서 번호, 금지 기법 등" />
          </div>
          <div className="md:col-span-2">
            <Button onClick={() => void save()}>
              <PlusIcon className="size-4" />
              저장
            </Button>
          </div>
        </CardContent>
      </Card>

      <div className="grid gap-3">
        {rows.map((row) => (
          <Card key={row.id}>
            <CardHeader className="pb-2">
              <div className="flex items-start justify-between gap-3">
                <div>
                  <CardTitle className="text-base">{row.name}</CardTitle>
                  <CardDescription>
                    {row.starts_at || "?"} ~ {row.ends_at || "?"} · {row.contact || "연락처 없음"}
                  </CardDescription>
                </div>
                <Button variant="ghost" size="icon" onClick={() => void remove(row.id)}>
                  <Trash2Icon className="size-4" />
                </Button>
              </div>
            </CardHeader>
            <CardContent className="grid gap-2 text-sm md:grid-cols-2">
              <div>
                <div className="text-xs text-muted-foreground">대상</div>
                <pre className="whitespace-pre-wrap font-mono text-xs">{row.targets || "-"}</pre>
              </div>
              <div>
                <div className="text-xs text-muted-foreground">제외</div>
                <pre className="whitespace-pre-wrap font-mono text-xs">{row.exclusions || "-"}</pre>
              </div>
              {row.notes ? <div className="md:col-span-2 text-muted-foreground">{row.notes}</div> : null}
            </CardContent>
          </Card>
        ))}
        {!loading && rows.length === 0 ? (
          <p className="text-sm text-muted-foreground">등록된 교전 범위가 없습니다.</p>
        ) : null}
      </div>
    </div>
  );
}
