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

  React.useEffect(() => {
    void loadScopes();
    void loadCred();
  }, [loadScopes, loadCred]);
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
                      <Badge variant={c.status === "evidence-backed" ? "default" : "secondary"}>{c.status}</Badge>
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
