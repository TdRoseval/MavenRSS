<script setup>
import { ref } from 'vue';
import SiteHeader from './components/SiteHeader.vue';
import HeroSection from './components/HeroSection.vue';
import FeatureSection from './components/FeatureSection.vue';
import DownloadSection from './components/DownloadSection.vue';
import Footer from './components/Footer.vue';

const isDarkHeader = ref(false);

const handleFeatureVisibility = (_) => {};

const handleFooterVisibility = (_) => {};
</script>

<template>
  <div class="landing-page">
    <SiteHeader :isDark="isDarkHeader" />
    <HeroSection />
    <FeatureSection @sectionVisibility="handleFeatureVisibility" />
    <DownloadSection />
    <Footer @footerVisibility="handleFooterVisibility" />
  </div>
</template>

<style scoped>
.landing-page {
  min-height: 100vh;
  background: linear-gradient(180deg, #f8f9fa 0%, #ffffff 100%);
}
</style>
