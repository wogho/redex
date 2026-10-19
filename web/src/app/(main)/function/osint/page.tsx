"use client";

import * as React from "react";

import { ImageIcon, RefreshCwIcon, SearchIcon, SendIcon, UsersIcon } from "lucide-react";
import { toast } from "sonner";

import { Badge } from "@/components/ui/badge";
import { Button } from "@/components/ui/button";
import { Card, CardContent, CardDescription, CardHeader, CardTitle } from "@/components/ui/card";
import { Input } from "@/components/ui/input";
import { Label } from "@/components/ui/label";
import { Select, SelectContent, SelectItem, SelectTrigger, SelectValue } from "@/components/ui/select";
import { api } from "@/lib/api";

type Candidate = {
  id: string;
  kind: string;
  value: string;
  source: string;
  detail: string;
  evidence?: string;
  confidence: number;
  status: string;
  created_at: string;
};

type Scope = { id: string; name: string; person_osint_allowed?: boolean };

const KINDS = [
  { id: "handle", label: "핸들" },
  { id: "email", label: "메일" },
  { id: "contact", label: "연락 수단" },
  { id: "profile", label: "프로필 수집" },
];

export default function OsintPage() {
  const [scopes, setScopes] = React.useState<Scope[]>([]);
  const [engagement, setEngagement] = React.useState("");
  const [kind, setKind] = React.useState("handle");
  const [value, setValue] = React.useState("");
  const [rows, setRows] = React.useState<Candidate[]>([]);
  const [busy, setBusy] = React.useState(false);
  // image enrolment / search
  const [label, setLabel] = React.useState("");
  const [file, setFile] = React.useState<File | null>(null);
  const [matches, setMatches] = React.useState<Array<{ id: string; label: string; score: number }>>([]);
  const [cred, setCred] = React.useState<{
    hibp_configured: boolean;
    ghunt_configured: boolean;
    instaloader_configured: boolean;
    instaloader_user: string;
  } | null>(null);
  const [hibpKey, setHibpKey] = React.useState("");
  const [hibpEmail, setHibpEmail] = React.useState("");
  const [igUser, setIgUser] = React.useState("");
  const [igPass, setIgPass] = React.useState("");
  const [credFile, setCredFile] = React.useState<File | null>(null);
  const [providers, setProviders] = React.useState<Array<{ id: string; name: string; category: string; fields: string[]; paid: boolean; docs: string; note?: string; configured: boolean; implemented: boolean }>>([]);
  const [providerVals, setProviderVals] = React.useState<Record<string, string>>({});
  const [providerInput, setProviderInput] = React.useState<Record<string, string>>({});
  const [packs, setPacks] = React.useState<Array<{ id: string; version: string; license: string; description: string; installed: boolean; enabled: boolean; pinned: boolean; size: number; bytes?: number }>>([]);
  const [packsMeta, setPacksMeta] = React.useState<{ dir: string; max_bytes: number; auto_evict: boolean; auto_delete_on_disable: boolean; used: number; free: number } | null>(null);
  const [packDir, setPackDir] = React.useState("");
  const [maxGB, setMaxGB] = React.useState("20");

  const loadScopes = React.useCallback(async () => {
    try {
      const r = await api.scopeList();
      setScopes((r.scopes ?? []) as unknown as Scope[]);
    } catch {
      /* login required */
    }
  }, []);

  const loadCandidates = React.useCallback(async () => {
    const q = new URLSearchParams();
    if (engagement) q.set("engagement_id", engagement);
    const r = await api.osintCandidates(q);
    setRows((r.candidates ?? []) as Candidate[]);
  }, [engagement]);

  const loadCred = React.useCallback(async () => {
    try {
      setCred(await api.osintCredentials());
    } catch {
      /* login required */
    }
  }, []);

  const loadProviders = React.useCallback(async () => {
    try {
      const r = await api.osintProviders();
      setProviders(r.providers ?? []);
    } catch {
      /* login required */
    }
  }, []);

  const saveProvider = async (id: string) => {
    const fields = providers.find((p) => p.id === id)?.fields ?? [];
    const creds: Record<string, string> = {};
    for (const f of fields) creds[f] = providerVals[`${id}.${f}`] ?? "";
    try {
      await api.osintProviderSave(id, creds);
      toast.success("저장됨");
      await loadProviders();
    } catch (err) {
      toast.error(err instanceof Error ? err.message : String(err));
    }
  };

  const runProvider = async (id: string) => {
    try {
      const r = await api.osintProviderRun(id, providerInput[id] ?? "");
      toast.success(`${r.count}건 수집`);
      await loadCandidates();
    } catch (err) {
      toast.error(err instanceof Error ? err.message : String(err));
    }
  };

  const loadPacks = React.useCallback(async () => {
    try {
      const r = await api.packsList();
      setPacks(r.packs ?? []);
      setPacksMeta({ ...(r.settings ?? {}), used: r.usage?.bytes ?? 0, free: r.usage?.free_bytes ?? 0 } as never);
      setPackDir((prev) => prev || r.settings?.dir || "");
    } catch {
      /* login required */
    }
  }, []);

  React.useEffect(() => {
    void loadScopes();
    void loadCred();
    void loadPacks();
    void loadProviders();
  }, [loadScopes, loadCred, loadPacks, loadProviders]);
  React.useEffect(() => {
    void loadCandidates();
  }, [loadCandidates]);

  const allowed = scopes.find((s) => s.id === engagement)?.person_osint_allowed === true;

  const run = async () => {
    if (!engagement) {
      toast.error("교전을 선택하세요");
      return;
    }
    if (!allowed) {
      toast.error("이 교전은 인물 조사가 허용되지 않았습니다");
      return;
    }
    setBusy(true);
    try {
      const r = await api.osintRun({ engagement_id: Number(engagement), kind, value, limit: 10 });
      toast.success(`${r.count}건 수집`);
      await loadCandidates();
    } catch (err) {
      toast.error(err instanceof Error ? err.message : String(err));
    } finally {
      setBusy(false);
    }
  };

  const saveHIBP = async () => {
    try {
      await api.osintSetHIBP(hibpKey);
      setHibpKey("");
      toast.success("HIBP 키 저장");
      await loadCred();
    } catch (err) {
      toast.error(err instanceof Error ? err.message : String(err));
    }
  };

  const runHIBP = async () => {
    try {
      const r = await api.osintHIBP(hibpEmail);
      toast.success(`브리치 ${r.count}건`);
      await loadCandidates();
    } catch (err) {
      toast.error(err instanceof Error ? err.message : String(err));
    }
  };

  const igLogin = async () => {
    try {
      const r = await api.osintInstaloaderLogin(igUser, igPass);
      setIgPass("");
      toast.success(`세션 저장: ${r.instaloader_user}`);
      await loadCred();
    } catch (err) {
      toast.error(err instanceof Error ? err.message : String(err));
    }
  };

  const uploadCred = async (kind: "ghunt" | "instaloader") => {
    if (!credFile) {
      toast.error("파일을 선택하세요");
      return;
    }
    try {
      if (kind === "ghunt") await api.osintGhuntUpload(credFile);
      else await api.osintInstaloaderSession(credFile);
      toast.success("저장 완료");
      setCredFile(null);
      await loadCred();
    } catch (err) {
      toast.error(err instanceof Error ? err.message : String(err));
    }
  };

  const packAction = async (id: string, action: "install" | "enable" | "disable" | "delete") => {
    try {
      if (action === "install") await api.packInstall(id);
      if (action === "enable") await api.packEnable(id);
      if (action === "disable") await api.packDisable(id);
      if (action === "delete") await api.packDelete(id);
      await loadPacks();
    } catch (err) {
      toast.error(err instanceof Error ? err.message : String(err));
    }
  };

  const togglePin = async (id: string, pinned: boolean) => {
    try {
      await api.packPin(id, pinned);
      await loadPacks();
    } catch (err) {
      toast.error(err instanceof Error ? err.message : String(err));
    }
  };

  const savePackSettings = async () => {
    try {
      await api.packSettings({ dir: packDir, max_bytes: Math.max(1, Number(maxGB) || 20) * 1024 * 1024 * 1024 });
      toast.success("팩 설정 저장");
      await loadPacks();
    } catch (err) {
      toast.error(err instanceof Error ? err.message : String(err));
    }
  };

  const verdict = async (id: string, status: string) => {
    try {
      await api.osintVerdict(id, status);
      await loadCandidates();
    } catch (err) {
      toast.error(err instanceof Error ? err.message : String(err));
    }
  };

  const enroll = async () => {
    if (!file || !label.trim()) {
      toast.error("이미지와 라벨이 필요합니다");
      return;
    }
    try {
      const r = await api.osintFaceIndex(file, label);
      toast.success(`등록 완료 (id ${r.id})`);
      setFile(null);
    } catch (err) {
      toast.error(err instanceof Error ? err.message : String(err));
    }
  };

  const search = async () => {
    if (!file) {
      toast.error("이미지를 선택하세요");
      return;
    }
    try {
      const r = await api.osintFaceSearch(file);
      setMatches(r.matches ?? []);
      if ((r.matches ?? []).length === 0) toast.info("일치 항목 없음");
    } catch (err) {
      toast.error(err instanceof Error ? err.message : String(err));
    }
  };

  return (
    <div className="flex flex-col gap-6 p-6">
      <div className="flex flex-wrap items-center justify-between gap-3">
        <div>
          <h1 className="flex items-center gap-2 text-2xl font-semibold">
            <UsersIcon className="size-6 text-primary" />
            OSINT / 정찰 보드
          </h1>
          <p className="mt-1 text-sm text-muted-foreground">
            공개 정보 수집과 로컬 이미지 매칭. 결과는 후보로 저장되며 검증 전에는 unverified 입니다.
          </p>
        </div>
        <div className="flex items-center gap-2">
          <Badge variant={allowed ? "default" : "destructive"}>
            {allowed ? "교전 허용됨" : "교전 미허용"}
          </Badge>
          <Button variant="outline" size="sm" onClick={() => void loadCandidates()}>
            <RefreshCwIcon className="size-4" />
            새로고침
          </Button>
        </div>
      </div>

      <Card>
        <CardHeader className="pb-3">
          <CardTitle className="text-base">수집 실행</CardTitle>
          <CardDescription>교전 범위(RoE)에서 인물 조사가 허용된 경우에만 실행됩니다.</CardDescription>
        </CardHeader>
        <CardContent className="flex flex-wrap items-end gap-3">
          <div className="flex flex-col gap-1">
            <Label>교전</Label>
            <Select value={engagement} onValueChange={setEngagement}>
              <SelectTrigger className="w-64">
                <SelectValue placeholder="교전 선택" />
              </SelectTrigger>
              <SelectContent>
                {scopes.map((s) => (
                  <SelectItem key={s.id} value={s.id}>
                    {s.name} {s.person_osint_allowed ? "" : "(미허용)"}
                  </SelectItem>
                ))}
              </SelectContent>
            </Select>
          </div>
          <div className="flex flex-col gap-1">
            <Label>수집 유형</Label>
            <Select value={kind} onValueChange={setKind}>
              <SelectTrigger className="w-40">
                <SelectValue />
              </SelectTrigger>
              <SelectContent>
                {KINDS.map((k) => (
                  <SelectItem key={k.id} value={k.id}>
                    {k.label}
                  </SelectItem>
                ))}
              </SelectContent>
            </Select>
          </div>
          <div className="flex min-w-[240px] flex-1 flex-col gap-1">
            <Label>{kind === "profile" ? "핸들" : "값"}</Label>
            <Input value={value} onChange={(e) => setValue(e.target.value)} placeholder={kind === "email" ? "user@example.com" : "handle"} />
          </div>
          <Button onClick={() => void run()} disabled={busy}>
            <SearchIcon className="size-4" />
            실행
          </Button>
        </CardContent>
      </Card>

      <Card>
        <CardHeader className="pb-3">
          <CardTitle className="text-base">연결 · 자격</CardTitle>
          <CardDescription>수집기 세션과 API 키를 여기서 저장합니다. 값은 다시 표시되지 않습니다.</CardDescription>
        </CardHeader>
        <CardContent className="grid gap-4 md:grid-cols-3">
          <div className="flex flex-col gap-2">
            <Label>HIBP 키</Label>
            <Badge variant={cred?.hibp_configured ? "default" : "outline"}>
              {cred?.hibp_configured ? "등록됨" : "없음"}
            </Badge>
            <Input placeholder="hibp api key" value={hibpKey} onChange={(e) => setHibpKey(e.target.value)} />
            <Button size="sm" variant="outline" onClick={() => void saveHIBP()}>키 저장</Button>
            <div className="flex gap-2">
              <Input placeholder="email" value={hibpEmail} onChange={(e) => setHibpEmail(e.target.value)} />
              <Button size="sm" onClick={() => void runHIBP()}>조회</Button>
            </div>
          </div>
          <div className="flex flex-col gap-2">
            <Label>GHunt 쿠키</Label>
            <Badge variant={cred?.ghunt_configured ? "default" : "outline"}>
              {cred?.ghunt_configured ? "등록됨" : "없음"}
            </Badge>
            <Input type="file" accept=".json,.txt" onChange={(e) => setCredFile(e.target.files?.[0] ?? null)} />
            <Button size="sm" variant="outline" onClick={() => void uploadCred("ghunt")}>쿠키 업로드</Button>
          </div>
          <div className="flex flex-col gap-2">
            <Label>instaloader 세션</Label>
            <Badge variant={cred?.instaloader_configured ? "default" : "outline"}>
              {cred?.instaloader_user ? `${cred.instaloader_user} 로그인됨` : "없음"}
            </Badge>
            <Input placeholder="username" value={igUser} onChange={(e) => setIgUser(e.target.value)} />
            <Input type="password" placeholder="password" value={igPass} onChange={(e) => setIgPass(e.target.value)} />
            <div className="flex gap-2">
              <Button size="sm" onClick={() => void igLogin()}>로그인</Button>
              <Button size="sm" variant="outline" onClick={() => void api.osintInstaloaderLogout().then(loadCred)}>로그아웃</Button>
            </div>
            <Input type="file" onChange={(e) => setCredFile(e.target.files?.[0] ?? null)} />
            <Button size="sm" variant="outline" onClick={() => void uploadCred("instaloader")}>세션 파일 업로드</Button>
          </div>
        </CardContent>
      </Card>

      <Card>
        <CardHeader className="pb-3">
          <CardTitle className="text-base">프로바이더 (키 입력)</CardTitle>
          <CardDescription>유료 포함 — 결제는 하지 않습니다. 키를 저장하면 해당 커넥터로 호출합니다.</CardDescription>
        </CardHeader>
        <CardContent className="flex flex-col gap-3">
          {providers.map((pv) => (
            <div key={pv.id} className="border-b pb-3 last:border-0">
              <div className="flex flex-wrap items-center gap-2">
                <span className="text-sm font-medium">{pv.name}</span>
                <span className="text-xs text-muted-foreground">{pv.category}</span>
                {pv.paid ? <Badge variant="outline">paid</Badge> : null}
                <Badge variant={pv.configured ? "default" : "outline"}>{pv.configured ? "키 등록" : "키 없음"}</Badge>
                {pv.note ? <span className="text-xs text-muted-foreground">{pv.note}</span> : null}
              </div>
              <div className="mt-2 flex flex-wrap items-end gap-2">
                {pv.fields.map((f) => (
                  <Input
                    key={f}
                    className="w-44"
                    placeholder={f}
                    type={f.includes("secret") || f.includes("key") || f.includes("cookie") || f.includes("bearer") ? "password" : "text"}
                    value={providerVals[`${pv.id}.${f}`] ?? ""}
                    onChange={(e) => setProviderVals({ ...providerVals, [`${pv.id}.${f}`]: e.target.value })}
                  />
                ))}
                <Button size="sm" variant="outline" onClick={() => void saveProvider(pv.id)} disabled={pv.fields.length === 0}>저장</Button>
                <Input
                  className="w-56"
                  placeholder="검색어"
                  value={providerInput[pv.id] ?? ""}
                  onChange={(e) => setProviderInput({ ...providerInput, [pv.id]: e.target.value })}
                />
                <Button size="sm" onClick={() => void runProvider(pv.id)} disabled={!pv.implemented}>실행</Button>
              </div>
            </div>
          ))}
        </CardContent>
      </Card>

      <Card>
        <CardHeader className="pb-3">
          <CardTitle className="text-base">소스 팩 (GitHub CDN)</CardTitle>
          <CardDescription>체크(설치·활성화) 시에만 내려받아 사용합니다. 모든 팩은 SHA256 검증 후 적용됩니다.</CardDescription>
        </CardHeader>
        <CardContent className="flex flex-col gap-2">
          {packs.map((pk) => (
            <div key={pk.id} className="flex flex-wrap items-center justify-between gap-2 border-b py-2 last:border-0">
              <div>
                <div className="text-sm font-medium">{pk.id} <span className="text-xs text-muted-foreground">v{pk.version} · {pk.license}</span></div>
                <div className="text-xs text-muted-foreground">{pk.description} · {(pk.size / 1024).toFixed(0)} KB</div>
              </div>
              <div className="flex gap-2">
                {!pk.installed ? (
                  <Button size="sm" variant="outline" onClick={() => void packAction(pk.id, "install")}>설치</Button>
                ) : (
                  <>
                    <Badge variant={pk.enabled ? "default" : "outline"}>{pk.enabled ? "활성" : "비활성"}</Badge>
                    <Button size="sm" variant="outline" onClick={() => void packAction(pk.id, pk.enabled ? "disable" : "enable")}>
                      {pk.enabled ? "비활성화" : "활성화"}
                    </Button>
                    <Button size="sm" variant="ghost" onClick={() => void togglePin(pk.id, !pk.pinned)}>{pk.pinned ? "고정됨" : "고정"}</Button>
                    <Button size="sm" variant="ghost" onClick={() => void packAction(pk.id, "delete")} disabled={pk.enabled}>삭제</Button>
                  </>
                )}
              </div>
            </div>
          ))}
          <div className="flex flex-wrap items-center justify-between gap-2 border-t pt-2 text-xs text-muted-foreground">
            <span>저장 경로: <span className="font-mono">{packsMeta?.dir ?? "-"}</span></span>
            <span>사용 {((packsMeta?.used ?? 0) / 1024 / 1024).toFixed(1)}MB / 여유 {((packsMeta?.free ?? 0) / 1024 / 1024 / 1024).toFixed(1)}GB</span>
          </div>
          <div className="flex flex-wrap items-end gap-2">
            <Input className="w-72" value={packDir} onChange={(e) => setPackDir(e.target.value)} placeholder="저장 경로" />
            <Input className="w-24" value={maxGB} onChange={(e) => setMaxGB(e.target.value.replace(/[^0-9]/g, ""))} placeholder="최대 GB" />
            <Button size="sm" variant="outline" onClick={() => void savePackSettings()}>경로·예산 저장</Button>
          </div>
          {packs.length === 0 ? <p className="text-sm text-muted-foreground">사용 가능한 팩이 없습니다.</p> : null}
        </CardContent>
      </Card>

      <div className="grid gap-4 lg:grid-cols-2">
        <Card>
          <CardHeader className="pb-3">
            <CardTitle className="flex items-center gap-2 text-base">
              <ImageIcon className="size-4" />
              이미지 등록
            </CardTitle>
            <CardDescription>라벨과 함께 임베딩을 저장합니다.</CardDescription>
          </CardHeader>
          <CardContent className="flex flex-wrap items-end gap-3">
            <Input type="file" accept="image/*" className="w-64" onChange={(e) => setFile(e.target.files?.[0] ?? null)} />
            <Input className="w-48" placeholder="라벨" value={label} onChange={(e) => setLabel(e.target.value)} />
            <Button variant="outline" onClick={() => void enroll()}>
              <SendIcon className="size-4" />
              등록
            </Button>
          </CardContent>
        </Card>

        <Card>
          <CardHeader className="pb-3">
            <CardTitle className="flex items-center gap-2 text-base">
              <SearchIcon className="size-4" />
              이미지 검색
            </CardTitle>
            <CardDescription>등록된 임베딩과 로컬에서 비교합니다(외부 전송 없음).</CardDescription>
          </CardHeader>
          <CardContent className="flex flex-col gap-3">
            <div className="flex flex-wrap items-end gap-3">
              <Input type="file" accept="image/*" className="w-64" onChange={(e) => setFile(e.target.files?.[0] ?? null)} />
              <Button variant="outline" onClick={() => void search()}>
                검색
              </Button>
            </div>
            {matches.length ? (
              <div className="text-sm">
                {matches.map((m) => (
                  <div key={`${m.id}-${m.score}`} className="flex justify-between border-b py-1 last:border-0">
                    <span className="font-mono text-xs">{m.label}</span>
                    <Badge variant="outline">{m.score.toFixed(3)}</Badge>
                  </div>
                ))}
              </div>
            ) : null}
          </CardContent>
        </Card>
      </div>

      <Card>
        <CardHeader className="pb-2">
          <CardTitle className="text-base">후보 ({rows.length})</CardTitle>
          <CardDescription>검증 전 후보입니다. 증거가 있는 항목만 evidence-backed 로 표시됩니다.</CardDescription>
        </CardHeader>
        <CardContent className="p-0">
          <div className="overflow-x-auto">
            <table className="w-full text-sm">
              <thead className="border-b bg-muted/40 text-left text-xs uppercase text-muted-foreground">
                <tr>
                  <th className="px-4 py-2">유형</th>
                  <th className="px-4 py-2">출처</th>
                  <th className="px-4 py-2">내용</th>
                  <th className="px-4 py-2">신뢰도</th>
                  <th className="px-4 py-2">상태</th>
                </tr>
              </thead>
              <tbody>
                {rows.map((c) => (
                  <tr key={c.id} className="border-b last:border-0">
                    <td className="px-4 py-2 font-mono text-xs">{c.kind}</td>
                    <td className="px-4 py-2 text-xs">{c.source}</td>
                    <td className="max-w-[420px] px-4 py-2">
                      <div className="truncate">{c.detail}</div>
                      <div className="truncate text-xs text-muted-foreground">{c.value}</div>
                    </td>
                    <td className="px-4 py-2 font-mono">{c.confidence}</td>
                    <td className="px-4 py-2">
                      <Badge variant={c.status === "evidence-backed" || c.status === "confirmed" ? "default" : "secondary"}>{c.status}</Badge>
                      <div className="mt-1 flex gap-1">
                        <Button size="sm" variant="ghost" onClick={() => void verdict(c.id, "confirmed")}>확정</Button>
                        <Button size="sm" variant="ghost" onClick={() => void verdict(c.id, "rejected")}>기각</Button>
                      </div>
                    </td>
                  </tr>
                ))}
                {rows.length === 0 ? (
                  <tr>
                    <td colSpan={5} className="px-4 py-10 text-center text-sm text-muted-foreground">
                      아직 수집된 후보가 없습니다.
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
