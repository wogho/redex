"use client";

import * as React from "react";

import { PlayIcon, RefreshCwIcon, SaveIcon, SlidersHorizontalIcon } from "lucide-react";
import { useTranslations } from "next-intl";
import { toast } from "sonner";

import { Badge } from "@/components/ui/badge";
import { Button } from "@/components/ui/button";
import { Card, CardContent, CardDescription, CardHeader, CardTitle } from "@/components/ui/card";
import { Input } from "@/components/ui/input";
import { Label } from "@/components/ui/label";
import { Textarea } from "@/components/ui/textarea";
import { api } from "@/lib/api";

export default function PolicyPage() {
  const t = useTranslations("policyPage");
  const [path, setPath] = React.useState("");
  const [configured, setConfigured] = React.useState(false);
  const [exists, setExists] = React.useState(false);
  const [content, setContent] = React.useState("");
  const [error, setError] = React.useState("");
  const [saving, setSaving] = React.useState(false);
  const [tool, setTool] = React.useState("Bash");
  const [command, setCommand] = React.useState("rm -rf /");
  const [testResult, setTestResult] = React.useState<{ blocked: boolean; reason: string } | null>(null);

  const load = React.useCallback(async () => {
    const v = await api.policyGet();
    setPath(v.path);
    setConfigured(v.configured);
    setExists(v.exists);
    setContent(v.content);
    setError(v.error ?? "");
  }, []);

  React.useEffect(() => {
    void load();
  }, [load]);

  const save = async () => {
    setSaving(true);
    try {
      await api.policyPut(content);
      toast.success(t("saved"));
      await load();
    } catch (err) {
      toast.error(err instanceof Error ? err.message : String(err));
    } finally {
      setSaving(false);
    }
  };

  const runTest = async () => {
    try {
      const r = await api.policyTest(tool, command);
      setTestResult(r);
    } catch (err) {
      toast.error(err instanceof Error ? err.message : String(err));
    }
  };

  return (
    <div className="flex flex-col gap-6 p-6">
      <div className="flex flex-wrap items-center justify-between gap-3">
        <div>
          <h1 className="flex items-center gap-2 text-2xl font-semibold">
            <SlidersHorizontalIcon className="size-6 text-primary" />
            {t("title")}
          </h1>
          <p className="mt-1 text-sm text-muted-foreground">{t("desc")}</p>
        </div>
        <div className="flex items-center gap-2">
          <Button variant="outline" size="sm" onClick={() => void load()}>
            <RefreshCwIcon className="size-4" />
            {t("reload")}
          </Button>
          <Button size="sm" onClick={() => void save()} disabled={!configured || saving}>
            <SaveIcon className="size-4" />
            {t("save")}
          </Button>
        </div>
      </div>

      <Card>
        <CardHeader className="pb-3">
          <CardTitle className="text-base">{t("path")}</CardTitle>
          <CardDescription className="font-mono text-xs">{path || t("not_configured")}</CardDescription>
        </CardHeader>
        <CardContent className="flex flex-wrap items-center gap-2 text-sm">
          <Badge variant={configured ? "default" : "destructive"}>
            {configured ? t("configured") : t("not_configured")}
          </Badge>
          <Badge variant="outline">{exists ? t("file_exists") : t("file_missing")}</Badge>
          {error ? <span className="text-destructive">{error}</span> : null}
        </CardContent>
      </Card>

      <Card>
        <CardHeader className="pb-3">
          <CardTitle className="text-base">{t("content")}</CardTitle>
          <CardDescription>{t("content_desc")}</CardDescription>
        </CardHeader>
        <CardContent>
          <Textarea
            className="min-h-[320px] font-mono text-xs"
            value={content}
            onChange={(e) => setContent(e.target.value)}
            spellCheck={false}
            placeholder={'{\n  "deny": [],\n  "allow": [],\n  "deny_tools": []\n}'}
          />
        </CardContent>
      </Card>

      <Card>
        <CardHeader className="pb-3">
          <CardTitle className="text-base">{t("test")}</CardTitle>
          <CardDescription>{t("test_desc")}</CardDescription>
        </CardHeader>
        <CardContent className="flex flex-col gap-3">
          <div className="flex flex-wrap items-end gap-3">
            <div className="flex flex-col gap-1">
              <Label htmlFor="policy-tool">{t("tool")}</Label>
              <Input id="policy-tool" className="w-40" value={tool} onChange={(e) => setTool(e.target.value)} />
            </div>
            <div className="flex min-w-[280px] flex-1 flex-col gap-1">
              <Label htmlFor="policy-command">{t("command")}</Label>
              <Input id="policy-command" value={command} onChange={(e) => setCommand(e.target.value)} />
            </div>
            <Button variant="outline" onClick={() => void runTest()}>
              <PlayIcon className="size-4" />
              {t("run")}
            </Button>
          </div>
          {testResult ? (
            <div className="flex items-center gap-2 text-sm">
              <Badge variant={testResult.blocked ? "destructive" : "default"}>
                {testResult.blocked ? t("blocked") : t("allowed")}
              </Badge>
              <span className="text-muted-foreground">{testResult.reason}</span>
            </div>
          ) : null}
        </CardContent>
      </Card>
    </div>
  );
}
