import { useEffect, useState } from "react";
import { Card, CardContent, CardDescription, CardHeader, CardTitle } from "@/components/ui/card";
import { type KeyStatus, fetchKeyStatus } from "@/lib/api";

// 凭据面板：只读掩码（绝不回显明文）；录入仅走 CLI 隐藏输入（SPEC §3.4 / 威胁模型）。
export function KeyPanel() {
  const [status, setStatus] = useState<KeyStatus | null>(null);
  const [error, setError] = useState("");

  useEffect(() => {
    fetchKeyStatus()
      .then(setStatus)
      .catch((e: unknown) => setError(String(e)));
  }, []);

  return (
    <Card className="max-w-xl">
      <CardHeader>
        <CardTitle>凭据状态</CardTitle>
        <CardDescription>API Key 只显示掩码与指纹；录入请在终端运行 gavel key set（隐藏输入）。</CardDescription>
      </CardHeader>
      <CardContent className="space-y-3 text-sm">
        {error && <p className="text-destructive">{error}</p>}
        <div className="flex items-center gap-3">
          <span className="w-20 shrink-0 text-muted-foreground">provider</span>
          <span className="font-mono">{status?.provider || "未配置"}</span>
        </div>
        <div className="flex items-center gap-3">
          <span className="w-20 shrink-0 text-muted-foreground">mask</span>
          <span className="rounded bg-muted px-2 py-1 font-mono">{status?.mask || "-"}</span>
        </div>
        <div className="flex items-center gap-3">
          <span className="w-20 shrink-0 text-muted-foreground">fingerprint</span>
          <span className="rounded bg-muted px-2 py-1 font-mono">{status?.fingerprint || "-"}</span>
        </div>
        <p className="text-xs text-muted-foreground">
          后端契约 GET /api/key/status → {"{provider, mask, fingerprint}"}，无明文字段。
        </p>
      </CardContent>
    </Card>
  );
}
