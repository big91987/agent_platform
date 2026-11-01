import { onScopeDispose, ref } from "vue";
import { fetchQuota, type Quota } from "./api";

export function useQuota(loader = fetchQuota) {
  const quota = ref<Quota | null>(null);
  const loading = ref(false);
  const error = ref("");
  let sequence = 0;
  let current: AbortController | undefined;
  async function load(tenant: string) {
    current?.abort();
    const request = ++sequence;
    current = new AbortController();
    const signal = current.signal;
    quota.value = null;
    error.value = "";
    loading.value = true;
    try {
      const value = await loader(tenant, signal);
      if (request === sequence && !signal.aborted) quota.value = value;
    } catch (cause) {
      if (request === sequence && !signal.aborted)
        error.value = cause instanceof Error ? cause.message : "加载失败";
    } finally {
      if (request === sequence) loading.value = false;
    }
  }
  onScopeDispose(() => {
    sequence++;
    current?.abort();
  });
  return { quota, loading, error, load };
}
