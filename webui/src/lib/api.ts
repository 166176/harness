// 后端契约（以 internal/server 实际实现为准，勿按 PLAN 旧描述）：
// - GET  /api/approvals/pending → {"approvals":[{id,sessionId,action:{Tool,Args},rule,risk,status,decidedBy}]}
// - POST /api/approvals/{id}     body {"decision":"approved"|"denied"}
// - GET  /api/events (SSE)：pending {type,id,sessionId,action,rule,risk,status} / decided {type,id,decision}，data: json\n\n
// - GET  /api/sessions → ["s1",...]；GET /api/sessions/{id} → core.Session（无 json tag，Go 字段名 ID/Repo/Task/TestCmd/State/Steps/MaxTurns）
// - GET  /api/key/status → {provider,mask,fingerprint}
// - GET  /api/demo → [{name,pass,trace}]（T14 cli.go 接线为小写键）

export interface ApiAction {
  Tool: string;
  Args?: Record<string, unknown>;
}

export interface Approval {
  id: string;
  sessionId: string;
  action: ApiAction;
  rule: string;
  risk: string;
  status: string;
  decidedBy?: string;
}

export type EventType = "pending" | "decided";

export interface PendingEvent {
  type: "pending";
  id: string;
  sessionId?: string;
  action?: ApiAction;
  rule?: string;
  risk?: string;
  status?: string;
}

export interface DecidedEvent {
  type: "decided";
  id: string;
  decision: string;
}

export type ApprovalEvent = PendingEvent | DecidedEvent;

export interface KeyStatus {
  provider: string;
  mask: string;
  fingerprint: string;
}

export interface DemoResult {
  name: string;
  pass: boolean;
  trace: string[];
}

export interface SessionStep {
  Seq: number;
  Decision: string;
  Rule: string;
  ToolName: string;
  Args?: Record<string, unknown> | null;
  Result: string;
  Feedback: string;
}

export interface Session {
  ID: string;
  Repo: string;
  Task: string;
  TestCmd: string;
  State: string;
  Steps?: SessionStep[] | null;
  MaxTurns: number;
}

async function fetchJSON<T>(url: string, init?: RequestInit): Promise<T> {
  const res = await fetch(url, init);
  if (!res.ok) {
    throw new Error(`${url} → HTTP ${res.status} ${res.statusText}`);
  }
  return (await res.json()) as T;
}

/** parseEvent 解析 SSE 的 data 行 JSON；未知类型抛错。 */
export function parseEvent(data: string): ApprovalEvent {
  const raw = JSON.parse(data) as { type?: unknown };
  if (raw.type !== "pending" && raw.type !== "decided") {
    throw new Error(`unknown SSE event type: ${String(raw.type)}`);
  }
  return raw as ApprovalEvent;
}

/** maskLabel 与后端 secret.Mask 语义一致：长度<=6 → "******"，否则前 3 字符 + "..." + 后 4 字符。 */
export function maskLabel(s: string): string {
  if (s.length <= 6) {
    return "******";
  }
  return s.slice(0, 3) + "..." + s.slice(-4);
}

/** fetchPendingApprovals 拉取 pending 审批；兼容大小写 envelope 键。 */
export async function fetchPendingApprovals(): Promise<Approval[]> {
  const data = await fetchJSON<{ approvals?: Approval[]; Approvals?: Approval[] }>("/api/approvals/pending");
  return data.approvals ?? data.Approvals ?? [];
}

/** decideApproval POST 审批决定（approved|denied）。 */
export async function decideApproval(id: string, decision: "approved" | "denied"): Promise<void> {
  await fetchJSON<unknown>(`/api/approvals/${encodeURIComponent(id)}`, {
    method: "POST",
    headers: { "Content-Type": "application/json" },
    body: JSON.stringify({ decision }),
  });
}

/** fetchSessions 返回会话 id 字符串数组。 */
export async function fetchSessions(): Promise<string[]> {
  const raw = await fetchJSON<unknown>("/api/sessions");
  return Array.isArray(raw) ? raw.map(String) : [];
}

/** fetchSession 返回单个会话（字段为 Go 名）。 */
export async function fetchSession(id: string): Promise<Session> {
  return fetchJSON<Session>(`/api/sessions/${encodeURIComponent(id)}`);
}

/** fetchKeyStatus 只回掩码/指纹，绝不回明文。 */
export async function fetchKeyStatus(): Promise<KeyStatus> {
  return fetchJSON<KeyStatus>("/api/key/status");
}

/** fetchDemo 拉取三场景演示结果；实际后端为小写键，兼容 PascalCase。 */
export async function fetchDemo(): Promise<DemoResult[]> {
  const raw = await fetchJSON<unknown>("/api/demo");
  const arr = Array.isArray(raw) ? raw : [];
  return arr.map((item) => {
    const r = item as Record<string, unknown>;
    const traceRaw = r.trace ?? r.Trace ?? [];
    return {
      name: String(r.name ?? r.Name ?? ""),
      pass: Boolean(r.pass ?? r.Pass ?? false),
      trace: Array.isArray(traceRaw) ? traceRaw.map(String) : [],
    };
  });
}

export interface EventHandlers {
  /** SSE 单条事件（pending/decided）。 */
  onEvent?: (e: ApprovalEvent) => void;
  /** 2s 轮询兜底返回的全量 pending 列表。 */
  onPendingList?: (list: Approval[]) => void;
}

/**
 * subscribeEvents 订阅 SSE 审批事件；EventSource 出错后启动 2s 轮询兜底。
 * 返回取消订阅函数（清理 EventSource 与定时器）。
 */
export function subscribeEvents(handlers: EventHandlers): () => void {
  let es: EventSource | null = null;
  let timer: number | null = null;

  const startPolling = () => {
    if (timer != null) {
      return;
    }
    timer = window.setInterval(async () => {
      try {
        handlers.onPendingList?.(await fetchPendingApprovals());
      } catch {
        // 后端暂时不可达：静默等待下一轮
      }
    }, 2000);
  };

  try {
    es = new EventSource("/api/events");
    es.onmessage = (ev) => {
      try {
        handlers.onEvent?.(parseEvent(ev.data));
      } catch {
        // 未知/非法事件：忽略
      }
    };
    es.onerror = () => {
      startPolling();
    };
  } catch {
    startPolling();
  }

  return () => {
    if (timer != null) {
      window.clearInterval(timer);
      timer = null;
    }
    es?.close();
  };
}
