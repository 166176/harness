import { useEffect, useState } from "react";
import { Badge } from "@/components/ui/badge";
import { Button } from "@/components/ui/button";
import { Card, CardContent, CardDescription, CardHeader, CardTitle } from "@/components/ui/card";
import { type DemoResult, fetchDemo } from "@/lib/api";

// 机制演示页：/api/demo 三场景（护栏拦截/反馈闭环/HITL 超时自动拒绝）PASS/FAIL 卡片。
export function DemoPage() {
  const [results, setResults] = useState<DemoResult[]>([]);
  const [error, setError] = useState("");
  const [loading, setLoading] = useState(false);

  const run = async () => {
    setLoading(true);
    setError("");
    try {
      setResults(await fetchDemo());
    } catch (e: unknown) {
      setError(String(e));
    } finally {
      setLoading(false);
    }
  };

  useEffect(() => {
    void run();
    // eslint-disable-next-line react-hooks/exhaustive-deps
  }, []);

  return (
    <div className="space-y-4">
      <div className="flex items-center justify-between">
        <p className="text-sm text-muted-foreground">
          调用 GET /api/demo（gavel demo 三场景，纯 MockLLM、确定性执行，不联网）。
        </p>
        <Button size="sm" variant="outline" disabled={loading} onClick={() => void run()}>
          {loading ? "执行中…" : "重新运行"}
        </Button>
      </div>
      {error && (
        <p className="rounded-md border border-destructive/40 bg-destructive/10 p-3 text-sm text-destructive">
          {error}
        </p>
      )}
      <div className="grid gap-4 md:grid-cols-3">
        {results.map((r) => (
          <Card key={r.name}>
            <CardHeader>
              <div className="flex items-center justify-between gap-2">
                <CardTitle className="truncate text-base">{r.name || "(未知场景)"}</CardTitle>
                <Badge variant={r.pass ? "success" : "destructive"}>{r.pass ? "PASS" : "FAIL"}</Badge>
              </div>
              <CardDescription>场景 {results.indexOf(r) + 1} / {results.length}</CardDescription>
            </CardHeader>
            <CardContent>
              <ul className="space-y-1 text-xs">
                {r.trace.map((line, i) => (
                  <li key={i} className={line.startsWith("FAIL") ? "text-destructive" : "text-muted-foreground"}>
                    {line}
                  </li>
                ))}
              </ul>
            </CardContent>
          </Card>
        ))}
      </div>
    </div>
  );
}
