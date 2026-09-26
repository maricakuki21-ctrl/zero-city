<template><AppLayout><main class="marketplace-page"><MarketExperience /></main></AppLayout></template>
<script setup lang="ts">
import AppLayout from '@/components/layout/AppLayout.vue'
import MarketExperience from '@/features/bizdecipher/components/marketplace/MarketExperience.vue'
</script>
<style scoped>.marketplace-page{width:min(1180px,100%);margin:0 auto;padding:20px}@media(max-width:680px){.marketplace-page{padding:12px}}</style>
