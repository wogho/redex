"use client";

import * as React from "react";

import { EyeIcon, KeyRoundIcon, PlusIcon, RefreshCwIcon, Trash2Icon } from "lucide-react";
import { toast } from "sonner";

import { Badge } from "@/components/ui/badge";
import { Button } from "@/components/ui/button";
import { Card, CardContent, CardDescription, CardHeader, CardTitle } from "@/components/ui/card";
import { Input } from "@/components/ui/input";
import { Label } from "@/components/ui/label";
import { api } from "@/lib/api";

type Cred = {
  id: string;
  name: string;
  kind: string;
  service: string;
  username: string;
  hint: string;
  notes: string;
  created_at: string;
};

export default function CredentialsPage() {
  const [rows, setRows] = React.useState<Cred[]>([]);
  const [name, setName] = React.useState("");
  const [kind, setKind] = React.useState("password");
  const [service, setService] = React.useState("");
  const [username, setUsername] = React.useState("");
  const [secret, setSecret] = React.useState("");
  const [notes, setNotes] = React.useState("");
  const [loading, setLoading] = React.useState(true);

  const load = React.useCallback(async () => {
    setLoading(true);
    try {
      const r = await api.credentialsList();
      setRows(r.credentials ?? []);
    } finally {
      setLoading(false);
    }
  }, []);

  React.useEffect(() => {
    void load();
  }, [load]);

  const create = async () => {
    if (!name.trim() || !secret) {
      toast.error("이름과 비밀값을 입력하세요");
      return;
    }
    try {
      await api.credentialCreate({ name, kind, service, username, secret, notes });
      setSecret("");
      toast.success("저장 완료 (암호화)");
      await load();
    } catch (err) {
      toast.error(err instanceof Error ? err.message : String(err));
    }
  };

  const reveal = async (id: string) => {
    try {
      const r = await api.credentialReveal(id);
      await navigator.clipboard.writeText(r.secret);
      toast.success("클립보드에 복사됨");
    } catch (err) {
      toast.error(err instanceof Error ? err.message : String(err));
    }
  };

  return (
    <div className="flex flex-col gap-6 p-6">
      <div className="flex flex-wrap items-center justify-between gap-3">
        <div>
          <h1 className="flex items-center gap-2 text-2xl font-semibold">
            <KeyRoundIcon className="size-6 text-primary" />
            자격증명 보관소
          </h1>
          <p className="mt-1 text-sm text-muted-foreground">
            비밀값은 AES-GCM으로 암호화되어 저장되며, 목록에는 마스킹된 힌트만 표시됩니다.
          </p>
        </div>
        <Button variant="outline" size="sm" onClick={() => void load()} disabled={loading}>
          <RefreshCwIcon className={loading ? "size-4 animate-spin" : "size-4"} />
          새로고침
        </Button>
      </div>

      <Card>
        <CardHeader className="pb-3">
          <CardTitle className="text-base">새 항목</CardTitle>
          <CardDescription>복사·조회는 명시적 동작이며 감사 대상입니다.</CardDescription>
        </CardHeader>
        <CardContent className="grid gap-3 md:grid-cols-3">
          <div className="flex flex-col gap-1"><Label>이름</Label><Input value={name} onChange={(e) => setName(e.target.value)} placeholder="DC01 로컬 관리자" /></div>
          <div className="flex flex-col gap-1"><Label>종류</Label><Input value={kind} onChange={(e) => setKind(e.target.value)} placeholder="password / hash / key" /></div>
          <div className="flex flex-col gap-1"><Label>서비스</Label><Input value={service} onChange={(e) => setService(e.target.value)} placeholder="smb / ssh / web" /></div>
          <div className="flex flex-col gap-1"><Label>사용자</Label><Input value={username} onChange={(e) => setUsername(e.target.value)} /></div>
          <div className="flex flex-col gap-1"><Label>비밀값</Label><Input type="password" value={secret} onChange={(e) => setSecret(e.target.value)} /></div>
          <div className="flex flex-col gap-1"><Label>메모</Label><Input value={notes} onChange={(e) => setNotes(e.target.value)} /></div>
          <div className="md:col-span-3"><Button onClick={() => void create()}><PlusIcon className="size-4" />저장</Button></div>
        </CardContent>
      </Card>

      <Card>
        <CardHeader className="pb-2">
          <CardTitle className="text-base">보관 항목 ({rows.length})</CardTitle>
          <CardDescription>비밀값은 목록에 표시되지 않습니다.</CardDescription>
        </CardHeader>
        <CardContent className="p-0">
          <div className="overflow-x-auto">
            <table className="w-full text-sm">
              <thead className="border-b bg-muted/40 text-left text-xs uppercase text-muted-foreground">
                <tr>
                  <th className="px-4 py-2">이름</th>
                  <th className="px-4 py-2">종류</th>
                  <th className="px-4 py-2">서비스</th>
                  <th className="px-4 py-2">사용자</th>
                  <th className="px-4 py-2">비밀값</th>
                  <th className="px-4 py-2">동작</th>
                </tr>
              </thead>
              <tbody>
                {rows.map((c) => (
                  <tr key={c.id} className="border-b last:border-0">
                    <td className="px-4 py-2 font-medium">{c.name}</td>
                    <td className="px-4 py-2"><Badge variant="outline">{c.kind || "-"}</Badge></td>
                    <td className="px-4 py-2 text-xs">{c.service || "-"}</td>
                    <td className="px-4 py-2 text-xs">{c.username || "-"}</td>
                    <td className="px-4 py-2 font-mono text-xs">{c.hint}</td>
                    <td className="px-4 py-2">
                      <div className="flex gap-1">
                        <Button size="sm" variant="ghost" onClick={() => void reveal(c.id)}><EyeIcon className="size-4" /></Button>
                        <Button size="sm" variant="ghost" onClick={async () => { await api.credentialDelete(c.id); await load(); }}><Trash2Icon className="size-4" /></Button>
                      </div>
                    </td>
                  </tr>
                ))}
                {rows.length === 0 ? (
                  <tr><td colSpan={6} className="px-4 py-10 text-center text-sm text-muted-foreground">저장된 자격증명이 없습니다.</td></tr>
                ) : null}
              </tbody>
            </table>
          </div>
        </CardContent>
      </Card>
    </div>
  );
}
