import { describe, it, expect } from "vitest";
import { parseEvent, maskLabel } from "../api";

describe("parseEvent", () => {
  it("解析 pending 事件", () => {
    expect(parseEvent('{"type":"pending","id":"a1","rule":"dangerous"}')).toEqual({ type: "pending", id: "a1", rule: "dangerous" });
  });
});

describe("maskLabel", () => {
  it("掩码永不等于原文", () => {
    const s = "sk-abcdefghij1234";
    expect(maskLabel(s)).not.toBe(s);
  });
});
