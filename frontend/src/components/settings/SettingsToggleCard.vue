<template>
  <div
    class="flex flex-col gap-3 rounded-xl border border-gray-100 bg-gray-50/70 p-4 transition-colors hover:border-primary-200 hover:bg-white dark:border-dark-700 dark:bg-dark-800/45 dark:hover:border-primary-500/40 dark:hover:bg-dark-800 sm:flex-row sm:items-center sm:justify-between"
  >
    <div class="min-w-0 pr-0 sm:pr-6">
      <label class="text-sm font-medium text-gray-700 dark:text-gray-300">
        {{ title }}
      </label>
      <p class="mt-0.5 text-xs text-gray-500 dark:text-gray-400">
        {{ hint }}
      </p>
      <p v-if="note" class="mt-2 text-xs text-amber-600 dark:text-amber-400">
        {{ note }}
      </p>
    </div>
    <Toggle
      :model-value="modelValue"
      class="shrink-0"
      @update:model-value="emit('update:modelValue', $event)"
    />
  </div>
</template>

<script setup lang="ts">
import Toggle from "@/components/common/Toggle.vue";

defineProps<{
  modelValue: boolean;
  title: string;
  hint: string;
  note?: string;
}>();

const emit = defineEmits<{
  "update:modelValue": [value: boolean];
}>();
</script>
