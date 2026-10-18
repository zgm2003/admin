<!-- 组件：页面容器 -->
<script setup lang="ts">
defineOptions({ name: 'AppPage' })
</script>

<template>
  <section class="app-page management-page">
    <slot />
  </section>
</template>
