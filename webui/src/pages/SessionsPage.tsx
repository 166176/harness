import { useEffect, useState } from "react";
import { Badge } from "@/components/ui/badge";
import { Button } from "@/components/ui/button";
import { Card, CardContent, CardHeader, CardTitle } from "@/components/ui/card";
import { type Session, type SessionStep, fetchSession, fetchSessions } from "@/lib/api";

function decisionBadge(decision: string) {
  switch (decision) {
    case "allow":
      return <Badge variant="success">allow</Badge>;
    case "deny":
      return <Badge variant="destructive">deny</Badge>;
    case "approval":
      return <Badge variant="warning">approval</Badge>;
    default:
      return <Badge variant="outline">{decision || "-"}</Badge>;
  }
}

function stateBadge(state: string) {
  switch (state) {
    case "completed":
      return <Badge variant="success">{state}</Badge>;
    case "failed":
      return <Badge variant="destructive">{state}</Badge>;
    case "terminated":
      return <Badge variant="warning">{state}</Badge>;
    default:
      return <Badge variant="outline">{state || "running"}</Badge>;
  }
}

function StepRow({ step }: { step: SessionStep }) {
  return (
    <div className="flex items-start gap-3 border-b py-2 text-sm last:border-b-0">
      <span className="w-8 shrink-0 text-right font-mono text-xs text-muted-foreground">#{step.Seq}</span>
      <span className="w-28 shrink-0 truncate font-mono text-xs">{step.ToolName || "-"}</span>
      <span className="w-24 shrink-0">{decisionBadge(step.Decision)}</span>
      <div className="min-w-0 flex-1 space-y-1">
        {step.Rule && <p className="font-mono text-xs text-muted-foreground">rule: {step.Rule}</p>}
        {step.Result && <p className="truncate text-xs" title={step.Result}>{step.Result}</p>}
        {step.Feedback && <p className="truncate text-xs text-amber-700" title={step.Feedback}>反馈: {step.Feedback}</p>}
      </div>
    </div>
  );
}

export function SessionsPage() {
  const [sessions, setSessions] = useState<string[]>([]);
  const [selectedId, setSelectedId] = useState("");
  const [detail, setDetail] = useState<Session | null>(null);
  const [error, setError] = useState("");

  useEffect(() => {
    let alive = true;
    const load = async () => {
      try {
        const list = await fetchSessions();
        if (alive) {
          setSessions(list);
          setError("");
        }
      } catch (e: unknown) {
        if (alive) {
          setError(String(e));
        }
      }
    };
    void load();
    const t = window.setInterval(() => void load(), 2000); // 会话监控兜底 2s 轮询
    return () => {
      alive = false;
      window.clearInterval(t);
    };
  }, []);

  useEffect(() => {
    if (!selectedId) {
      return;
    }
    let alive = true;
    const load = async () => {
      try {
        const d = await fetchSession(selectedId);
        if (alive) {
          setDetail(d);
          setError("");
        }
      } catch (e: unknown) {
        if (alive) {
          setError(String(e));
        }
      }
    };
    void load();
    const t = window.setInterval(() => void load(), 2000);
    return () => {
      alive = false;
      window.clearInterval(t);
    };
  }, [selectedId]);

  const steps = detail?.Steps ?? [];

  return (
    <div className="grid gap-4 lg:grid-cols-[260px_1fr]">
      <Card>
        <CardHeader>
          <CardTitle className="text-base">会话</CardTitle>
        </CardHeader>
        <CardContent>
          {error && <p className="mb-2 text-xs text-destructive">{error}</p>}
          {sessions.length === 0 && <p className="text-sm text-muted-foreground">暂无会话</p>}
          <ul className="space-y-1">
            {sessions.map((id) => (
              <li key={id}>
                <Button
                  variant={selectedId === id ? "secondary" : "ghost"}
                  size="sm"
                  className="w-full justify-start font-mono"
                  onClick={() => setSelectedId(id)}
                >
                  {id}
                </Button>
              </li>
            ))}
          </ul>
        </CardContent>
      </Card>
      <Card>
        <CardHeader>
          <div className="flex items-center justify-between gap-2">
            <CardTitle className="truncate font-mono text-base">{detail?.ID ?? "未选择会话"}</CardTitle>
            {detail && stateBadge(detail.State)}
          </div>
          {detail && (
            <p className="truncate text-sm text-muted-foreground">
              repo={detail.Repo} · task={detail.Task} · test={detail.TestCmd} · maxTurns={detail.MaxTurns}
            </p>
          )}
        </CardHeader>
        <CardContent>
          {steps.length === 0 && <p className="text-sm text-muted-foreground">暂无 Step 流水。</p>}
          {steps.map((step, i) => (
            <StepRow key={i} step={step} />
          ))}
        </CardContent>
      </Card>
    </div>
  );
}
