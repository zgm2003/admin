<script setup lang="ts">
import AppServerOverview from '@/components/AppServerOverview/index.vue'
</script>

<template>
  <div class="server-page"><AppServerOverview /></div>
</template>

<style scoped lang="scss">
.server-page {
  min-width: 0;
  padding: 24px;
  max-width: 1280px;
  margin: 0 auto;
}
@media (max-width: 600px) {
  .server-page {
    padding: 12px;
  }
}
</style>
