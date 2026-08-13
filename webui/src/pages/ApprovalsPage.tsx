import { useEffect, useRef, useState } from "react";
import { Badge } from "@/components/ui/badge";
import { Button } from "@/components/ui/button";
import { Card, CardContent, CardDescription, CardFooter, CardHeader, CardTitle } from "@/components/ui/card";
import { Dialog, DialogFooter } from "@/components/ui/dialog";
import {
  type ApiAction,
  type Approval,
  type PendingEvent,
  decideApproval,
  fetchPendingApprovals,
  subscribeEvents,
} from "@/lib/api";

// 倒计时优先取后端 createdAt（T16 起接口透出）+ 默认策略 approval_timeout_seconds=300；
// 旧后端/无 createdAt 时回退到“本页首次观测到该审批”起算的近似值。
const NOMINAL_TIMEOUT_SECONDS = 300;

function fmtCountdown(secondsLeft: number): string {
  const s = Math.max(0, secondsLeft);
  const mm = Math.floor(s / 60);
  const ss = s % 60;
  return `${String(mm).padStart(2, "0")}:${String(ss).padStart(2, "0")}`;
}

function secondsLeftFor(a: Approval, now: number, seenAt: Map<string, number>): number {
  if (a.createdAt) {
    const created = Date.parse(a.createdAt);
    if (!Number.isNaN(created)) {
      return NOMINAL_TIMEOUT_SECONDS - Math.floor((now - created) / 1000);
    }
  }
  const seen = seenAt.get(a.id) ?? now;
  return NOMINAL_TIMEOUT_SECONDS - Math.floor((now - seen) / 1000);
}

function argsSummary(action: ApiAction): string {
  const args = action.Args ?? {};
  const s = JSON.stringify(args);
  return s.length > 200 ? s.slice(0, 200) + "…" : s;
}

function approvalFromEvent(ev: PendingEvent): Approval {
  return {
    id: ev.id,
    sessionId: ev.sessionId ?? "",
    action: ev.action ?? { Tool: "" },
    rule: ev.rule ?? "",
    risk: ev.risk ?? "",
    status: ev.status ?? "pending",
    createdAt: ev.createdAt,
  };
}

export function ApprovalsPage() {
  const [approvals, setApprovals] = useState<Approval[]>([]);
  const [error, setError] = useState("");
  const [busyId, setBusyId] = useState("");
  const [terminating, setTerminating] = useState<Approval | null>(null);
  const [now, setNow] = useState(() => Date.now());
  const seenAt = useRef<Map<string, number>>(new Map());

  useEffect(() => {
    fetchPendingApprovals()
      .then((list) => {
        const t = Date.now();
        for (const a of list) {
          if (!seenAt.current.has(a.id)) {
            seenAt.current.set(a.id, t);
          }
        }
        setApprovals(list);
      })
      .catch((e: unknown) => setError(String(e)));

    const off = subscribeEvents({
      onEvent: (ev) => {
        if (ev.type === "pending") {
          setApprovals((prev) => {
            if (prev.some((a) => a.id === ev.id)) {
              return prev;
            }
            const t = Date.now();
            if (!seenAt.current.has(ev.id)) {
              seenAt.current.set(ev.id, t);
            }
            return [...prev, approvalFromEvent(ev)];
          });
        } else {
          setApprovals((prev) => prev.filter((a) => a.id !== ev.id));
        }
      },
      onPendingList: (list) => {
        const t = Date.now();
        for (const a of list) {
          if (!seenAt.current.has(a.id)) {
            seenAt.current.set(a.id, t);
          }
        }
        setApprovals(list);
      },
    });

    const tick = window.setInterval(() => setNow(Date.now()), 1000);
    return () => {
      off();
      window.clearInterval(tick);
    };
  }, []);

  const decide = async (id: string, decision: "approved" | "denied") => {
    setBusyId(id);
    setError("");
    try {
      await decideApproval(id, decision);
      setApprovals((prev) => prev.filter((a) => a.id !== id));
    } catch (e: unknown) {
      setError(String(e));
    } finally {
      setBusyId("");
    }
  };

  return (
    <div className="space-y-4">
      {error && (
        <p className="rounded-md border border-destructive/40 bg-destructive/10 p-3 text-sm text-destructive">
          {error}
        </p>
      )}
      {approvals.length === 0 && (
        <Card>
          <CardHeader>
            <CardTitle>审批队列</CardTitle>
            <CardDescription>暂无待审批动作。</CardDescription>
          </CardHeader>
        </Card>
      )}
      {approvals.map((a) => {
        const secondsLeft = secondsLeftFor(a, now, seenAt.current);
        return (
          <Card key={a.id}>
            <CardHeader>
              <div className="flex items-center justify-between gap-2">
                <CardTitle className="font-mono text-base">{a.action.Tool || "(未知工具)"}</CardTitle>
                <Badge variant={secondsLeft <= 60 ? "warning" : "outline"} className="font-mono">
                  ⏱ {fmtCountdown(secondsLeft)}
                </Badge>
              </div>
              <CardDescription className="font-mono text-xs">
                id={a.id} · session={a.sessionId || "-"}
              </CardDescription>
            </CardHeader>
            <CardContent className="space-y-2 text-sm">
              <div className="flex flex-wrap items-center gap-2">
                <span className="text-muted-foreground">命中规则</span>
                <Badge variant="destructive">{a.rule || "-"}</Badge>
                <span className="text-muted-foreground">风险</span>
                <span>{a.risk || "-"}</span>
              </div>
              <pre className="overflow-x-auto rounded-md bg-muted p-3 font-mono text-xs">
                {argsSummary(a.action)}
              </pre>
            </CardContent>
            <CardFooter className="gap-2">
              <Button size="sm" disabled={busyId === a.id} onClick={() => void decide(a.id, "approved")}>
                批准
              </Button>
              <Button
                size="sm"
                variant="destructive"
                disabled={busyId === a.id}
                onClick={() => void decide(a.id, "denied")}
              >
                拒绝
              </Button>
              <Button size="sm" variant="outline" disabled={busyId === a.id} onClick={() => setTerminating(a)}>
                拒绝并终止
              </Button>
            </CardFooter>
          </Card>
        );
      })}

      <Dialog
        open={terminating !== null}
        onOpenChange={(open) => {
          if (!open) {
            setTerminating(null);
          }
        }}
        title="拒绝并终止"
        description={
          terminating
            ? `确定拒绝「${terminating.action.Tool}」并终止会话 ${terminating.sessionId || "-"}？当前后端 API 仅支持 approved|denied，本操作将拒绝该动作；会话级终止暂无独立 REST 通道（见任务报告 concern）。`
            : ""
        }
      >
        <DialogFooter>
          <Button variant="outline" onClick={() => setTerminating(null)}>
            取消
          </Button>
          <Button
            variant="destructive"
            onClick={() => {
              if (terminating) {
                void decide(terminating.id, "denied");
              }
              setTerminating(null);
            }}
          >
            确认拒绝
          </Button>
        </DialogFooter>
      </Dialog>
    </div>
  );
}
