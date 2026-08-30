<script setup lang="ts">
import { ref } from 'vue'
import { Eye, EyeOff } from 'lucide-vue-next'
import { useI18n } from 'vue-i18n'
import { Input } from '@/components/ui/input'

// inheritAttrs: false + the explicit v-bind="$attrs" below is what lets a
// caller pass through arbitrary native input attributes (inputmode,
// maxlength, class, @keydown.enter, ...) straight onto the real <input> —
// this component's root used to be the wrapping <div>, so those attrs
// landed on a div instead (autocomplete/inputmode/maxlength do nothing
// there, and a keydown handler only worked by accident, via bubbling).
defineOptions({ inheritAttrs: false })

withDefaults(
  defineProps<{
    modelValue: string
    placeholder?: string
    disabled?: boolean
    autocomplete?: string
  }>(),
  { placeholder: '', disabled: false, autocomplete: 'off' },
)
const emit = defineEmits<{ 'update:modelValue': [string] }>()
const { t } = useI18n()

const visible = ref(false)
</script>

<template>
  <div class="relative">
    <Input
      v-bind="$attrs"
      :model-value="modelValue"
      :type="visible ? 'text' : 'password'"
      :autocomplete="autocomplete"
      :placeholder="placeholder"
      :disabled="disabled"
      class="pr-9 font-mono"
      @update:model-value="(v) => emit('update:modelValue', String(v))"
    />
    <button
      type="button"
      class="absolute right-2 top-1/2 -translate-y-1/2 text-muted-foreground hover:text-foreground disabled:opacity-50"
      :disabled="disabled"
      :aria-label="visible ? t('settings.secretInput.hide') : t('settings.secretInput.show')"
      @click="visible = !visible"
    >
      <EyeOff v-if="visible" class="w-4 h-4" />
      <Eye v-else class="w-4 h-4" />
    </button>
  </div>
</template>
