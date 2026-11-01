<script setup lang="ts">
import { onMounted, ref } from "vue";
import { useQuota } from "./useQuota";
const tenant = ref("demo");
const { quota, loading, error, load } = useQuota();
function refresh() {
  if (tenant.value.trim()) void load(tenant.value.trim());
}
onMounted(refresh);
</script>

<template>
  <section aria-labelledby="quota-title">
    <header>
      <h1 id="quota-title">配额管理</h1>
      <p>查看租户的配额使用情况</p>
    </header>
    <form @submit.prevent="refresh">
      <label for="tenant">租户</label>
      <div class="controls">
        <input id="tenant" v-model="tenant" required /><button
          type="submit"
          :disabled="!tenant.trim()"
        >
          查询
        </button>
      </div>
    </form>
    <p v-if="loading" role="status">正在加载…</p>
    <p v-else-if="error" role="alert">
      {{ error }} <button type="button" @click="refresh">重试</button>
    </p>
    <dl v-else-if="quota" class="metrics">
      <div>
        <dt>总配额</dt>
        <dd>{{ quota.limit }}</dd>
      </div>
      <div>
        <dt>已使用</dt>
        <dd>{{ quota.used }}</dd>
      </div>
      <div>
        <dt>已预留</dt>
        <dd>{{ quota.reserved }}</dd>
      </div>
      <div>
        <dt>可用</dt>
        <dd>{{ quota.limit - quota.used - quota.reserved }}</dd>
      </div>
    </dl>
  </section>
</template>
