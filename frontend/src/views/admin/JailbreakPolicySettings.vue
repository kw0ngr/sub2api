<template>
  <div class="space-y-6">
    <div class="flex items-center justify-between">
      <div>
        <h3 class="text-lg font-medium text-gray-900 dark:text-gray-100">
          破限链策略（Grok）
        </h3>
        <p class="text-sm text-gray-500 dark:text-gray-400 mt-1">
          策略 JSON 存 settings 表，60s TTL 热更新；grok-lab 导出后直接粘贴导入。留空保存 = 恢复内置默认配方。
        </p>
      </div>
      <div class="flex gap-2">
        <button type="button" class="btn btn-secondary" :disabled="jbOperating" @click="loadPolicy">
          刷新
        </button>
        <button type="button" class="btn btn-secondary" :disabled="jbOperating || !jbRaw" @click="exportPolicy">
          导出
        </button>
        <button
          type="button"
          class="btn btn-danger"
          :disabled="jbOperating"
          @click="resetPolicy"
        >
          恢复默认
        </button>
      </div>
    </div>

    <div v-if="jbLoading" class="text-sm text-gray-500">加载中…</div>
    <template v-else>
      <div>
        <label class="block text-sm font-medium text-gray-700 dark:text-gray-300 mb-1">
          策略 JSON
        </label>
        <textarea
          v-model="jbRaw"
          rows="18"
          spellcheck="false"
          class="w-full rounded-lg border border-gray-300 dark:border-dark-600 bg-white dark:bg-dark-700 px-3 py-2 font-mono text-xs leading-relaxed focus:outline-none focus:ring-2 focus:ring-primary-500"
          placeholder='{"schema_version":1,...}'
        />
      </div>

      <div v-if="jbMessage" :class="jbMessageClass" class="rounded-lg px-3 py-2 text-sm">
        {{ jbMessage }}
      </div>

      <div class="flex items-center justify-between">
        <p class="text-xs text-gray-500 dark:text-gray-400">
          保存前会先调 validate 干跑校验（正则可编译 / 模板占位符），失败不落库。
        </p>
        <div class="flex gap-2">
          <button type="button" class="btn btn-secondary" :disabled="jbOperating" @click="validatePolicy">
            校验
          </button>
          <button type="button" class="btn btn-primary" :disabled="jbOperating" @click="savePolicy">
            保存
          </button>
        </div>
      </div>

      <div v-if="jbMeta" class="rounded-lg border border-gray-200 dark:border-dark-600 p-4 text-xs text-gray-600 dark:text-gray-400 space-y-1">
        <p><span class="font-medium">schema_version：</span>{{ jbMeta.schema_version }}</p>
        <p><span class="font-medium">system：</span>{{ jbMeta.system_enabled ? '启用' : '关闭' }}（{{ jbMeta.system_len }} 字）</p>
        <p><span class="font-medium">remap 类目：</span>{{ jbMeta.remap_categories }} 个</p>
        <p><span class="font-medium">detect 词表：</span>P1 {{ jbMeta.detect_p1 }} / P2 {{ jbMeta.detect_p2 }} / DZ {{ jbMeta.detect_dz }}</p>
        <p><span class="font-medium">ladder：</span>max_rounds={{ jbMeta.ladder_max_rounds }}，follow_offer={{ jbMeta.follow_offer_enabled ? 'on' : 'off' }}，frame_shift={{ jbMeta.frame_shift_enabled ? 'on' : 'off' }}</p>
      </div>
    </template>
  </div>
</template>

<script setup lang="ts">
import { onMounted, ref } from 'vue'
import { useAppStore } from '@/stores/app'
import { apiClient } from '@/api/client'

const appStore = useAppStore()

const jbLoading = ref(true)
const jbOperating = ref(false)
const jbRaw = ref('')
const jbMessage = ref('')
const jbMessageClass = ref('')
const jbMeta = ref<null | {
  schema_version: number
  system_enabled: boolean
  system_len: number
  remap_categories: number
  detect_p1: number
  detect_p2: number
  detect_dz: number
  ladder_max_rounds: number
  follow_offer_enabled: boolean
  frame_shift_enabled: boolean
}>(null)

function extractMeta(policy: any) {
  if (!policy) return null
  return {
    schema_version: policy.schema_version,
    system_enabled: !!policy.system?.enabled,
    system_len: (policy.system?.text || '').length,
    remap_categories: policy.remap?.categories?.length ?? 0,
    detect_p1: policy.detect?.p1?.length ?? 0,
    detect_p2: policy.detect?.p2?.length ?? 0,
    detect_dz: policy.detect?.deadzone?.length ?? 0,
    ladder_max_rounds: policy.ladder?.max_rounds ?? 0,
    follow_offer_enabled: !!policy.ladder?.follow_offer?.enabled,
    frame_shift_enabled: !!policy.ladder?.frame_shift?.enabled,
  }
}

function flash(msg: string, kind: 'ok' | 'err') {
  jbMessage.value = msg
  jbMessageClass.value =
    kind === 'ok'
      ? 'bg-green-50 text-green-700 dark:bg-green-900/30 dark:text-green-300'
      : 'bg-red-50 text-red-700 dark:bg-red-900/30 dark:text-red-300'
}

async function loadPolicy() {
  jbLoading.value = true
  try {
    const { data } = await apiClient.get('/admin/settings/jailbreak-grok-policy')
    jbRaw.value = data?.data?.raw ?? ''
    jbMeta.value = extractMeta(data?.data?.policy)
  } catch (e: any) {
    flash('加载失败：' + (e?.response?.data?.message || e.message), 'err')
  } finally {
    jbLoading.value = false
  }
}

async function validatePolicy() {
  jbOperating.value = true
  jbMessage.value = ''
  try {
    const { data } = await apiClient.post(
      '/admin/settings/jailbreak-grok-policy/validate',
      { raw: jbRaw.value },
    )
    jbMeta.value = extractMeta(data?.data?.policy)
    flash('校验通过（未落库）', 'ok')
  } catch (e: any) {
    flash('校验失败：' + (e?.response?.data?.message || e.message), 'err')
  } finally {
    jbOperating.value = false
  }
}

async function savePolicy() {
  jbOperating.value = true
  jbMessage.value = ''
  try {
    const { data } = await apiClient.put(
      '/admin/settings/jailbreak-grok-policy',
      { raw: jbRaw.value },
    )
    jbMeta.value = extractMeta(data?.data?.policy)
    const { data: fresh } = await apiClient.get('/admin/settings/jailbreak-grok-policy')
    jbRaw.value = fresh?.data?.raw ?? jbRaw.value
    flash('已保存并生效（缓存已失效，立即热更新）', 'ok')
    appStore.showSuccess('破限策略已保存')
  } catch (e: any) {
    flash('保存失败：' + (e?.response?.data?.message || e.message), 'err')
  } finally {
    jbOperating.value = false
  }
}

async function resetPolicy() {
  jbOperating.value = true
  jbMessage.value = ''
  try {
    await apiClient.delete('/admin/settings/jailbreak-grok-policy')
    await loadPolicy()
    flash('已恢复内置默认配方', 'ok')
  } catch (e: any) {
    flash('恢复失败：' + (e?.response?.data?.message || e.message), 'err')
  } finally {
    jbOperating.value = false
  }
}

function exportPolicy() {
  const blob = new Blob([jbRaw.value], { type: 'application/json' })
  const url = URL.createObjectURL(blob)
  const a = document.createElement('a')
  a.href = url
  a.download = `jailbreak-grok-policy-${new Date().toISOString().slice(0, 10)}.json`
  a.click()
  URL.revokeObjectURL(url)
}

onMounted(loadPolicy)
</script>
