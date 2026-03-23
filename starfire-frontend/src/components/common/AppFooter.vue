<template>
  <footer class="app-footer">
    <div class="footer-container">
      <div class="footer-left">
        <span class="system-status">
          <span class="status-dot"></span>
          系统状态: <span class="status-text">正常</span>
        </span>
        <span class="data-sync">
          数据同步: <span class="sync-time">{{ lastSyncTime }}</span>
        </span>
      </div>
      <div class="footer-right">
        <span class="copyright">星火量化 © 2024</span>
      </div>
    </div>
  </footer>
</template>

<script setup>
import { ref, onMounted, onUnmounted } from 'vue'
import { formatTime } from '@/utils/formatters'

const lastSyncTime = ref('--')
let timer = null

onMounted(() => {
  updateSyncTime()
  timer = setInterval(updateSyncTime, 60000)
})

onUnmounted(() => {
  if (timer) clearInterval(timer)
})

const updateSyncTime = () => {
  lastSyncTime.value = formatTime(Date.now())
}
</script>

<style lang="scss" scoped>
@import '@/assets/styles/variables.scss';

.app-footer {
  background: $surface;
  border-top: 1px solid $border;
  padding: 12px 24px;
  margin-top: auto;

  .footer-container {
    max-width: 1400px;
    margin: 0 auto;
    display: flex;
    justify-content: space-between;
    align-items: center;
    font-size: 12px;
    color: $text-tertiary;
  }

  .footer-left {
    display: flex;
    gap: 32px;

    .system-status {
      display: flex;
      align-items: center;
      gap: 6px;

      .status-dot {
        width: 8px;
        height: 8px;
        border-radius: 50%;
        background: $success;
      }

      .status-text {
        color: $success;
      }
    }
  }

  .copyright {
    color: $text-tertiary;
  }
}
</style>
