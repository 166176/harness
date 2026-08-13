import { Tabs, TabsContent, TabsList, TabsTrigger } from "@/components/ui/tabs";
import { ApprovalsPage } from "@/pages/ApprovalsPage";
import { DemoPage } from "@/pages/DemoPage";
import { KeyPanel } from "@/pages/KeyPanel";
import { SessionsPage } from "@/pages/SessionsPage";

export default function App() {
  return (
    <div className="min-h-screen bg-background">
      <header className="border-b">
        <div className="mx-auto max-w-5xl px-4 py-4">
          <h1 className="text-2xl font-bold">gavel · 审批控制台</h1>
          <p className="text-sm text-muted-foreground">治理护栏 · 人工审批 · 会话监控 · 机制演示</p>
        </div>
      </header>
      <main className="mx-auto max-w-5xl px-4 py-6">
        <Tabs defaultValue="approvals">
          <TabsList>
            <TabsTrigger value="approvals">审批控制台</TabsTrigger>
            <TabsTrigger value="sessions">会话监控</TabsTrigger>
            <TabsTrigger value="key">凭据面板</TabsTrigger>
            <TabsTrigger value="demo">机制演示</TabsTrigger>
          </TabsList>
          <TabsContent value="approvals">
            <ApprovalsPage />
          </TabsContent>
          <TabsContent value="sessions">
            <SessionsPage />
          </TabsContent>
          <TabsContent value="key">
            <KeyPanel />
          </TabsContent>
          <TabsContent value="demo">
            <DemoPage />
          </TabsContent>
        </Tabs>
      </main>
    </div>
  );
}
