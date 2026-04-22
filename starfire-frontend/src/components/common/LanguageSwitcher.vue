<template>
  <el-dropdown trigger="click" @command="handleCommand">
    <span class="language-trigger">
      <el-icon><More /></el-icon>
      <span class="current-lang">{{ currentLangLabel }}</span>
      <el-icon class="arrow"><ArrowDown /></el-icon>
    </span>
    <template #dropdown>
      <el-dropdown-menu>
        <el-dropdown-item
          v-for="lang in languages"
          :key="lang.code"
          :command="lang.code"
          :class="{ 'is-active': locale === lang.code }"
        >
          <span class="lang-option">
            <span class="lang-flag">{{ lang.flag }}</span>
            <span class="lang-label">{{ lang.label }}</span>
          </span>
        </el-dropdown-item>
      </el-dropdown-menu>
    </template>
  </el-dropdown>
</template>

<script setup>
import { computed } from 'vue'
import { useI18n } from 'vue-i18n'
import { More, ArrowDown } from '@element-plus/icons-vue'

const { locale } = useI18n()

const languages = [
  { code: 'zh', label: '简体中文', flag: '🇨🇳' },
  { code: 'en', label: 'English', flag: '🇺🇸' }
]

const currentLangLabel = computed(() => {
  const lang = languages.find(l => l.code === locale.value)
  return lang ? lang.label : '中文'
})

const handleCommand = (code) => {
  locale.value = code
  localStorage.setItem('locale', code)

  // Update URL if using locale in path
  updateUrlLocale(code)
}

const updateUrlLocale = (code) => {
  const path = window.location.pathname
  const hash = window.location.hash

  // Check if URL already has locale prefix
  const localePattern = /^\/(zh|en)/
  if (localePattern.test(path)) {
    // Replace locale in path
    const newPath = path.replace(localePattern, `/${code}`)
    window.history.pushState({}, '', newPath + hash)
  }
}
</script>

<style scoped>
.language-trigger {
  display: flex;
  align-items: center;
  gap: 4px;
  cursor: pointer;
  padding: 4px 8px;
  border-radius: 4px;
  color: var(--el-text-color-regular);
  transition: all 0.2s;

  &:hover {
    background-color: var(--el-fill-color-light);
  }
}

.arrow {
  font-size: 12px;
}

.lang-option {
  display: flex;
  align-items: center;
  gap: 8px;
}

.lang-flag {
  font-size: 16px;
}

.lang-label {
  font-size: 14px;
}

:deep(.el-dropdown-menu__item.is-active) {
  background-color: var(--el-color-primary-light-9);
  color: var(--el-color-primary);
}
</style>
