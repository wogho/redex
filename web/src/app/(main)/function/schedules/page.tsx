"use client";

import * as React from "react";

import { CalendarClockIcon, PlayIcon, PlusIcon, RefreshCwIcon, Trash2Icon } from "lucide-react";
import { toast } from "sonner";

import { Badge } from "@/components/ui/badge";
import { Button } from "@/components/ui/button";
import { Card, CardContent, CardDescription, CardHeader, CardTitle } from "@/components/ui/card";
import { Input } from "@/components/ui/input";
import { Label } from "@/components/ui/label";
import { Select, SelectContent, SelectItem, SelectTrigger, SelectValue } from "@/components/ui/select";
import { Switch } from "@/components/ui/switch";
import { api } from "@/lib/api";

type Schedule = {
  id: string;
  task_id: string;
  name: string;
  goal: string;
  interval_minutes: number;
  enabled: boolean;
  last_run_at: string;
  next_run_at: string;
};

export default function SchedulesPage() {
  const [rows, setRows] = React.useState<Schedule[]>([]);
  const [tasks, setTasks] = React.useState<Array<{ id: string; name?: string; description?: string }>>([]);
  const [taskId, setTaskId] = React.useState("");
  const [name, setName] = React.useState("");
  const [goal, setGoal] = React.useState("");
  const [interval, setIntervalMin] = React.useState("60");
  const [enabled, setEnabled] = React.useState(true);
  const [loading, setLoading] = React.useState(true);

  const load = React.useCallback(async () => {
    setLoading(true);
    try {
      const [s, t] = await Promise.all([api.schedulesList(), api.tasks()]);
      setRows(s.schedules ?? []);
      setTasks(t.tasks as unknown as Array<{ id: string; name?: string; description?: string }>);
    } finally {
      setLoading(false);
    }
  }, []);

  React.useEffect(() => {
    void load();
  }, [load]);

  const create = async () => {
    if (!taskId || !goal.trim()) {
      toast.error("태스크와 목표를 입력하세요");
      return;
    }
    try {
      await api.scheduleCreate({
        task_id: taskId,
        name: name || "정기 재실행",
        goal: goal.trim(),
        interval_minutes: Number(interval) || 60,
        enabled,
      });
      setGoal("");
      toast.success("스케줄 저장");
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
            <CalendarClockIcon className="size-6 text-primary" />
            스케줄 / 지속 테스트
          </h1>
          <p className="mt-1 text-sm text-muted-foreground">
            분 단위로 태스크에 목표를 추가해 재실행합니다(내부 API 호출). 결과는 사용 현황·보고서에 누적됩니다.
          </p>
        </div>
        <Button variant="outline" size="sm" onClick={() => void load()} disabled={loading}>
          <RefreshCwIcon className={loading ? "size-4 animate-spin" : "size-4"} />
          새로고침
        </Button>
      </div>

      <Card>
        <CardHeader className="pb-3">
          <CardTitle className="text-base">새 스케줄</CardTitle>
          <CardDescription>목표 텍스트가 매 실행마다 추가됩니다.</CardDescription>
        </CardHeader>
        <CardContent className="flex flex-wrap items-end gap-3">
          <div className="flex w-64 flex-col gap-1">
            <Label>태스크</Label>
            <Select value={taskId} onValueChange={setTaskId}>
              <SelectTrigger><SelectValue placeholder="태스크 선택" /></SelectTrigger>
              <SelectContent>
                {tasks.map((t) => (
                  <SelectItem key={t.id} value={t.id}>
                    {t.name || t.description || `#${t.id}`}
                  </SelectItem>
                ))}
              </SelectContent>
            </Select>
          </div>
          <div className="flex w-48 flex-col gap-1">
            <Label>이름</Label>
            <Input value={name} onChange={(e) => setName(e.target.value)} placeholder="야간 재점검" />
          </div>
          <div className="flex min-w-[260px] flex-1 flex-col gap-1">
            <Label>목표</Label>
            <Input value={goal} onChange={(e) => setGoal(e.target.value)} placeholder="재점검 후 신규 취약점 보고" />
          </div>
          <div className="flex w-28 flex-col gap-1">
            <Label>간격(분)</Label>
            <Input value={interval} onChange={(e) => setIntervalMin(e.target.value.replace(/[^0-9]/g, ""))} />
          </div>
          <div className="flex items-center gap-2 pb-2">
            <Switch checked={enabled} onCheckedChange={setEnabled} />
            <span className="text-sm">활성</span>
          </div>
          <Button onClick={() => void create()}>
            <PlusIcon className="size-4" />
            저장
          </Button>
        </CardContent>
      </Card>

      <Card>
        <CardHeader className="pb-2">
          <CardTitle className="text-base">스케줄 ({rows.length})</CardTitle>
          <CardDescription>다음 실행 시각은 UTC 기준입니다.</CardDescription>
        </CardHeader>
        <CardContent className="p-0">
          <div className="overflow-x-auto">
            <table className="w-full text-sm">
              <thead className="border-b bg-muted/40 text-left text-xs uppercase text-muted-foreground">
                <tr>
                  <th className="px-4 py-2">이름</th>
                  <th className="px-4 py-2">태스크</th>
                  <th className="px-4 py-2">간격</th>
                  <th className="px-4 py-2">다음 실행</th>
                  <th className="px-4 py-2">상태</th>
                  <th className="px-4 py-2">동작</th>
                </tr>
              </thead>
              <tbody>
                {rows.map((r) => (
                  <tr key={r.id} className="border-b last:border-0">
                    <td className="px-4 py-2">
                      <div className="font-medium">{r.name || "-"}</div>
                      <div className="text-xs text-muted-foreground">{r.goal}</div>
                    </td>
                    <td className="px-4 py-2 font-mono text-xs">{r.task_id}</td>
                    <td className="px-4 py-2">{r.interval_minutes}분</td>
                    <td className="px-4 py-2 text-xs">{r.next_run_at ? new Date(r.next_run_at).toLocaleString() : "-"}</td>
                    <td className="px-4 py-2">
                      <Badge variant={r.enabled ? "default" : "outline"}>{r.enabled ? "활성" : "중지"}</Badge>
                    </td>
                    <td className="px-4 py-2">
                      <div className="flex gap-1">
                        <Button size="sm" variant="ghost" onClick={async () => { await api.scheduleRun(r.id); toast.success("실행"); await load(); }}>
                          <PlayIcon className="size-4" />
                        </Button>
                        <Button size="sm" variant="ghost" onClick={async () => { await api.scheduleToggle(r.id, !r.enabled); await load(); }}>
                          {r.enabled ? "중지" : "활성"}
                        </Button>
                        <Button size="sm" variant="ghost" onClick={async () => { await api.scheduleDelete(r.id); await load(); }}>
                          <Trash2Icon className="size-4" />
                        </Button>
                      </div>
                    </td>
                  </tr>
                ))}
                {rows.length === 0 ? (
                  <tr><td colSpan={6} className="px-4 py-10 text-center text-sm text-muted-foreground">스케줄이 없습니다.</td></tr>
                ) : null}
              </tbody>
            </table>
          </div>
        </CardContent>
      </Card>
    </div>
  );
}
