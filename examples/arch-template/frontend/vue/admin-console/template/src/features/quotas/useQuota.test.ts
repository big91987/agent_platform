import { effectScope } from "vue";
import { describe, expect, it } from "vitest";
import { useQuota } from "./useQuota";
import type { Quota } from "./api";

describe("quota state ownership", () => {
  it("a late response cannot overwrite a newer tenant or clear its loading state", async () => {
    const pending = new Map<string, (value: Quota) => void>();
    const signals: AbortSignal[] = [];
    const scope = effectScope();
    const state = scope.run(() =>
      useQuota((tenant, signal) => {
        signals.push(signal);
        return new Promise<Quota>((resolve) => pending.set(tenant, resolve));
      }),
    )!;
    const first = state.load("one");
    const second = state.load("two");
    expect(signals[0]?.aborted).toBe(true);
    pending.get("one")!({ tenant: "one", limit: 10, reserved: 0, used: 1 });
    await first;
    expect(state.quota.value).toBeNull();
    expect(state.loading.value).toBe(true);
    pending.get("two")!({ tenant: "two", limit: 20, reserved: 0, used: 2 });
    await second;
    expect(state.quota.value?.tenant).toBe("two");
    expect(state.loading.value).toBe(false);
    scope.stop();
  });

  it("disposing a feature cancels its request and does not display its eventual error", async () => {
    let reject!: (error: Error) => void;
    let signal!: AbortSignal;
    const scope = effectScope();
    const state = scope.run(() =>
      useQuota((_tenant, inputSignal) => {
        signal = inputSignal;
        return new Promise<Quota>((_resolve, fail) => {
          reject = fail;
        });
      }),
    )!;
    const loading = state.load("one");
    scope.stop();
    expect(signal.aborted).toBe(true);
    reject(new Error("late network error"));
    await loading;
    expect(state.error.value).toBe("");
  });
});
