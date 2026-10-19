import * as React from "react";

import { Card, CardContent, CardDescription, CardHeader, CardTitle } from "@/components/ui/card";
import { Badge } from "@/components/ui/badge";

export function RoadmapPage({
  title,
  desc,
  bullets,
  note,
}: {
  title: string;
  desc: string;
  bullets: string[];
  note?: string;
}) {
  return (
    <div className="flex flex-col gap-6 p-6">
      <div>
        <h1 className="flex items-center gap-2 text-2xl font-semibold">
          {title}
          <Badge variant="outline">soon</Badge>
        </h1>
        <p className="mt-1 text-sm text-muted-foreground">{desc}</p>
      </div>

      <Card>
        <CardHeader className="pb-2">
          <CardTitle className="text-base">계획</CardTitle>
          <CardDescription>아직 구현 전입니다. 아래 항목이 들어갑니다.</CardDescription>
        </CardHeader>
        <CardContent>
          <ul className="list-disc space-y-2 pl-5 text-sm text-muted-foreground">
            {bullets.map((b) => (
              <li key={b}>{b}</li>
            ))}
          </ul>
        </CardContent>
      </Card>

      {note ? <p className="text-sm text-muted-foreground">{note}</p> : null}
    </div>
  );
}
