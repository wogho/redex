"use client";

import * as React from "react";

import { Button } from "@/components/ui/button";
import { Checkbox } from "@/components/ui/checkbox";
import { Dialog, DialogContent, DialogDescription, DialogFooter, DialogHeader, DialogTitle } from "@/components/ui/dialog";
import { Label } from "@/components/ui/label";

export function ConsentDialog({
  open,
  onOpenChange,
  title,
  description,
  terms,
  confirmLabel = "동의하고 활성화",
  onConfirm,
}: {
  open: boolean;
  onOpenChange: (v: boolean) => void;
  title: string;
  description?: string;
  terms: string[];
  confirmLabel?: string;
  onConfirm: () => void | Promise<void>;
}) {
  const [checked, setChecked] = React.useState(false);
  const [busy, setBusy] = React.useState(false);

  React.useEffect(() => {
    if (open) setChecked(false);
  }, [open]);

  const agree = async () => {
    setBusy(true);
    try {
      await onConfirm();
      onOpenChange(false);
    } finally {
      setBusy(false);
    }
  };

  return (
    <Dialog open={open} onOpenChange={onOpenChange}>
      <DialogContent className="max-w-2xl">
        <DialogHeader>
          <DialogTitle>{title}</DialogTitle>
          {description ? <DialogDescription>{description}</DialogDescription> : null}
        </DialogHeader>
        <div className="max-h-[50vh] space-y-2 overflow-y-auto rounded-md border bg-muted/30 p-4 text-sm">
          {terms.map((line) => (
            <p key={line} className="text-muted-foreground">
              {line}
            </p>
          ))}
        </div>
        <Label className="flex items-start gap-2 text-sm font-normal">
          <Checkbox checked={checked} onCheckedChange={(v) => setChecked(v === true)} />
          <span>위 조항을 읽었으며, 이 팩/프로바이더의 사용 조건과 책임 범위에 동의합니다.</span>
        </Label>
        <DialogFooter>
          <Button variant="outline" onClick={() => onOpenChange(false)} disabled={busy}>
            취소
          </Button>
          <Button onClick={() => void agree()} disabled={!checked || busy}>
            {confirmLabel}
          </Button>
        </DialogFooter>
      </DialogContent>
    </Dialog>
  );
}
