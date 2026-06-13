<script setup lang="ts">
import DefaultTheme from 'vitepress/theme'

const { Layout } = DefaultTheme
</script>

<template>
  <Layout>
    <template #home-hero-before>
      <div class="gs-hero-kicker">Spring-style development experience for Go</div>
    </template>
  </Layout>
</template>
