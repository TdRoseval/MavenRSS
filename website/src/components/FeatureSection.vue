<script setup>
import { h, ref, watch, onMounted, onUnmounted } from 'vue';
import {
  PhRobot,
  PhMagnifyingGlass,
  PhWrench,
  PhFingerprint,
  PhClock,
  PhFileText,
  PhPlus,
  PhFilePlus,
  PhFunnel,
  PhShootingStar,
  PhCheck,
  PhUser,
  PhCloudSlash,
  PhCpu,
} from '@phosphor-icons/vue';

const activeFeatureIndex = ref(0);
const scrollProgress = ref(0);
const isAnimating = ref(false);
const animationStep = ref(0);
const chatMessages = ref([{ role: 'user', text: 'Summarize this article' }]);
const currentSummary = ref('');
const showHeaderBackground = ref(false);
const isMobileOrSmallScreen = ref(false);
const emit = defineEmits(['sectionVisibility']);

// Check if screen is mobile or too small for animations
const checkScreenSize = () => {
  const width = window.innerWidth;
  const height = window.innerHeight;
  // Disable animations on mobile or small screens (height < 980px)
  isMobileOrSmallScreen.value = width < 1024 || height < 980;
};

// Watch for feature changes to reset animation
watch(activeFeatureIndex, () => {
  isAnimating.value = false;
  animationStep.value = 0;
  resetDemos();
});

const resetDemos = () => {
  chatMessages.value = [{ role: 'user', text: 'Summarize this article' }];
  currentSummary.value = '';
};

// AI Chat Demo
const toggleAIChat = () => {
  isAnimating.value = !isAnimating.value;
  if (isAnimating.value) {
    // Simulate typing
    setTimeout(() => {
      currentSummary.value =
        'This article discusses breakthrough AI developments including machine learning advances, real-world applications in healthcare and finance, and future implications for society.';
    }, 1500);
  } else {
    currentSummary.value = '';
  }
};

// Feed Discovery Demo
const discoveredFeeds = ref([]);
const toggleDiscovery = () => {
  isAnimating.value = !isAnimating.value;
  if (isAnimating.value) {
    discoveredFeeds.value = [];
    const feeds = [
      { name: 'TechCrunch', count: 1234, icon: 'TC' },
      { name: 'The Verge', count: 892, icon: 'TV' },
    ];
    feeds.forEach((feed, index) => {
      setTimeout(
        () => {
          discoveredFeeds.value.push(feed);
        },
        (index + 1) * 600
      );
    });
  } else {
    discoveredFeeds.value = [];
  }
};

// Workflow Demo
const toggleWorkflow = () => {
  isAnimating.value = !isAnimating.value;
  if (isAnimating.value) {
    animationStep.value = 0;
    const steps = [1, 2, 3, 4];
    steps.forEach((step, index) => {
      setTimeout(
        () => {
          animationStep.value = step;
        },
        (index + 1) * 800
      );
    });
  } else {
    animationStep.value = 0;
  }
};

// Privacy Demo
const privacyVerified = ref(false);
const togglePrivacy = () => {
  isAnimating.value = !isAnimating.value;
  if (isAnimating.value) {
    privacyVerified.value = false;
    setTimeout(() => {
      privacyVerified.value = true;
    }, 1200);
  } else {
    privacyVerified.value = false;
  }
};

const features = [
  {
    title: 'AI Intelligent Processing',
    description:
      'Transform your reading experience with advanced AI capabilities that understand, summarize, and translate content in real-time.',
    highlights: [
      'Instant article summaries',
      'Multi-language translation',
      'Chat with your content',
      'Smart content extraction',
    ],
    colorClass: 'blue',
    accentColor: '#007bff',
    icon: () => h(PhRobot, { size: 48 }),
  },
  {
    title: 'Smart Feed Discovery',
    description:
      'Never run out of great content. Our AI intelligently discovers and recommends RSS feeds based on your interests.',
    highlights: [
      'Automatic feed suggestions',
      'Topic-based discovery',
      'Import from OPML',
      'Category organization',
    ],
    colorClass: 'purple',
    accentColor: '#FFC107',
    icon: () => h(PhMagnifyingGlass, { size: 48 }),
  },
  {
    title: 'Automation & Workflows',
    description:
      'Create powerful automated workflows to process, filter, and act on your RSS content automatically.',
    highlights: [
      'Custom scripting support',
      'Rule-based filtering',
      'Auto-tagging system',
      'Webhook integrations',
    ],
    colorClass: 'yellow',
    accentColor: '#FFC107',
    icon: () => h(PhWrench, { size: 48 }),
  },
  {
    title: '100% Local Privacy',
    description:
      'Your data never leaves your device. All AI processing happens locally on your machine for complete privacy.',
    highlights: [
      'Offline AI processing',
      'No cloud dependencies',
      'Your data stays yours',
      'Open source & transparent',
    ],
    colorClass: 'green',
    accentColor: '#28a745',
    icon: () => h(PhFingerprint, { size: 48 }),
  },
];

const handleScroll = () => {
  const section = document.querySelector('.features-section');
  if (!section) return;

  const rect = section.getBoundingClientRect();
  const windowHeight = window.innerHeight;
  const sectionHeight = rect.height;
  const sectionTop = section.offsetTop;
  const scrollY = window.scrollY;

  // 使用更可靠的滚动进度计算
  const start = sectionTop - windowHeight;
  const end = sectionTop + sectionHeight;
  const rawProgress = (scrollY - start) / (end - start);

  // 限制进度在0-1之间
  const progress = Math.max(0, Math.min(1, rawProgress));
  scrollProgress.value = progress;

  // 当滚动到最后一个功能结束时(进度接近1),显示标题背景
  showHeaderBackground.value = progress > 0.9;

  // 根据进度确定当前激活的功能索引
  const featureIndex = Math.floor(progress * features.length);
  const newIndex = Math.min(featureIndex, features.length - 1);

  if (newIndex !== activeFeatureIndex.value) {
    activeFeatureIndex.value = newIndex;
  }

  // 通知父组件当前section底部是否离开视窗超过一半
  const viewportHeight = window.innerHeight;
  const isBottomAboveHalfViewport = rect.bottom < viewportHeight * 0.5;
  emit('sectionVisibility', isBottomAboveHalfViewport);
};

const throttledScroll = (() => {
  let ticking = false;
  return () => {
    if (!ticking) {
      window.requestAnimationFrame(() => {
        handleScroll();
        ticking = false;
      });
      ticking = true;
    }
  };
})();

const scrollToFeature = (index) => {
  const section = document.querySelector('.features-section');
  if (!section) return;

  const sectionTop = section.offsetTop;
  const sectionHeight = section.offsetHeight;
  const windowHeight = window.innerHeight;

  // 计算目标滚动位置，与handleScroll中的progress计算一致
  const start = sectionTop - windowHeight;
  const end = sectionTop + sectionHeight;
  const targetProgress = index / features.length;
  const targetScroll = start + targetProgress * (end - start);

  window.scrollTo({
    top: targetScroll,
    behavior: 'smooth',
  });
};

onMounted(() => {
  checkScreenSize();
  window.addEventListener('scroll', throttledScroll, { passive: true });
  window.addEventListener('resize', checkScreenSize);
  setTimeout(() => handleScroll(), 100);
});

onUnmounted(() => {
  window.removeEventListener('scroll', throttledScroll);
  window.removeEventListener('resize', checkScreenSize);
});
</script>

<template>
  <section id="features" class="features-section">
    <div class="section-background">
      <div class="bg-gradient-1"></div>
      <div class="bg-gradient-2"></div>
    </div>

    <div class="container">
      <div
        class="section-header-sticky"
        :class="{ 'has-background': showHeaderBackground, 'no-sticky': isMobileOrSmallScreen }"
      >
        <div class="section-header">
          <h2 class="section-title">
            <span class="title-outline">Powerful</span>
            <span class="title-fill">Capabilities</span>
          </h2>
          <p class="section-description">
            Everything you need in a modern RSS reader, enhanced with cutting-edge AI technology
          </p>
        </div>
      </div>

      <div class="features-scroll-area" :class="{ 'compact-mode': isMobileOrSmallScreen }">
        <div class="features-layout" :class="{ 'stacked-layout': isMobileOrSmallScreen }">
          <div class="feature-visual-sticky" v-show="!isMobileOrSmallScreen">
            <div class="feature-visual-large">
              <div class="visual-container">
                <div class="app-interface">
                  <div class="interface-header">
                    <div class="interface-dots">
                      <span></span>
                      <span></span>
                      <span></span>
                    </div>
                    <span class="interface-title">MavenRSS</span>
                  </div>
                  <div class="interface-body">
                    <div class="sidebar">
                      <button
                        class="sidebar-item"
                        :class="{ active: activeFeatureIndex === 0 }"
                        @click="scrollToFeature(0)"
                        :title="features[0].title"
                      >
                        <div class="sidebar-icon"></div>
                      </button>
                      <button
                        class="sidebar-item"
                        :class="{ active: activeFeatureIndex === 1 }"
                        @click="scrollToFeature(1)"
                        :title="features[1].title"
                      >
                        <div class="sidebar-icon"></div>
                      </button>
                      <button
                        class="sidebar-item"
                        :class="{ active: activeFeatureIndex === 2 }"
                        @click="scrollToFeature(2)"
                        :title="features[2].title"
                      >
                        <div class="sidebar-icon"></div>
                      </button>
                      <button
                        class="sidebar-item"
                        :class="{ active: activeFeatureIndex === 3 }"
                        @click="scrollToFeature(3)"
                        :title="features[3].title"
                      >
                        <div class="sidebar-icon"></div>
                      </button>
                    </div>
                    <div class="main-content">
                      <!-- Feature 0: AI Intelligent Processing -->
                      <div v-if="activeFeatureIndex === 0" class="demo-content ai-demo">
                        <div class="demo-header">
                          <div class="demo-badge">AI Assistant</div>
                          <div class="demo-status" :class="{ active: isAnimating }">
                            <span class="status-dot"></span>
                            {{ isAnimating ? 'Processing...' : 'Ready' }}
                          </div>
                        </div>
                        <div class="chat-container">
                          <div
                            v-for="(msg, idx) in chatMessages"
                            :key="idx"
                            class="chat-message"
                            :class="msg.role === 'user' ? 'user-message' : 'ai-message'"
                          >
                            <div v-if="msg.role === 'user'" class="message-avatar">
                              <PhUser :size="24" />
                            </div>
                            <div v-else class="message-avatar">
                              <PhRobot :size="24" />
                            </div>
                            <div class="message-bubble">{{ msg.text }}</div>
                          </div>
                          <div
                            v-if="isAnimating && !currentSummary"
                            class="chat-message ai-message typing"
                          >
                            <div class="message-avatar">
                              <PhRobot :size="24" />
                            </div>
                            <div class="message-bubble">
                              <div class="typing-indicator">
                                <span></span>
                                <span></span>
                                <span></span>
                              </div>
                            </div>
                          </div>
                          <div v-if="currentSummary" class="chat-message ai-message appear">
                            <div class="message-avatar">
                              <PhRobot :size="24" />
                            </div>
                            <div class="message-bubble">
                              <div class="summary-content">{{ currentSummary }}</div>
                              <div class="summary-stats">
                                <div class="stat-item">
                                  <PhClock :size="14" />
                                  <span>2s saved</span>
                                </div>
                                <div class="stat-item">
                                  <PhFileText :size="14" />
                                  <span>85% shorter</span>
                                </div>
                              </div>
                            </div>
                          </div>
                        </div>
                        <button
                          class="demo-btn"
                          @click="toggleAIChat"
                          :disabled="isAnimating && !currentSummary"
                        >
                          <span v-if="!isAnimating">Generate Summary</span>
                          <span v-else-if="!currentSummary">Generating...</span>
                          <span v-else>Reset</span>
                        </button>
                      </div>

                      <!-- Feature 1: Smart Feed Discovery -->
                      <div v-if="activeFeatureIndex === 1" class="demo-content discovery-demo">
                        <div class="demo-header">
                          <div class="demo-badge">Feed Discovery</div>
                          <div class="demo-status" :class="{ active: isAnimating }">
                            <span class="status-dot"></span>
                            {{
                              discoveredFeeds.length > 0
                                ? `${discoveredFeeds.length} found`
                                : isAnimating
                                  ? 'Searching...'
                                  : 'Ready'
                            }}
                          </div>
                        </div>
                        <div class="discovery-container">
                          <div class="search-box" :class="{ searching: isAnimating }">
                            <div class="search-icon">
                              <PhMagnifyingGlass :size="14" />
                            </div>
                            <div class="search-input">Technology blogs</div>
                            <div
                              v-if="isAnimating && discoveredFeeds.length === 0"
                              class="search-pulse"
                            ></div>
                          </div>
                          <div class="feed-results">
                            <TransitionGroup name="feed">
                              <div
                                v-for="(feed, idx) in discoveredFeeds"
                                :key="feed.name"
                                class="feed-item"
                                :style="{ transitionDelay: `${idx * 100}ms` }"
                              >
                                <div class="feed-icon">{{ feed.icon }}</div>
                                <div class="feed-info">
                                  <div class="feed-name">{{ feed.name }}</div>
                                  <div class="feed-count">
                                    {{ feed.count.toLocaleString() }} articles
                                  </div>
                                </div>
                                <button class="feed-add" @click.stop>
                                  <PhPlus :size="16" weight="bold" />
                                </button>
                              </div>
                            </TransitionGroup>
                          </div>
                        </div>
                        <button class="demo-btn" @click="toggleDiscovery">
                          <span v-if="!isAnimating">Discover Feeds</span>
                          <span v-else>Reset</span>
                        </button>
                      </div>

                      <!-- Feature 2: Automation & Workflows -->
                      <div v-if="activeFeatureIndex === 2" class="demo-content automation-demo">
                        <div class="demo-header">
                          <div class="demo-badge">Automation</div>
                          <div class="demo-status" :class="{ active: isAnimating }">
                            <span class="status-dot"></span>
                            {{
                              animationStep >= 4
                                ? 'Completed'
                                : isAnimating
                                  ? `Step ${animationStep}/4`
                                  : 'Ready'
                            }}
                          </div>
                        </div>
                        <div class="workflow-container">
                          <div
                            class="workflow-step"
                            :class="{ active: animationStep >= 1, done: animationStep > 1 }"
                          >
                            <div class="step-icon">
                              <PhFilePlus :size="24" />
                            </div>
                            <div class="step-label">New Article</div>
                          </div>
                          <div class="workflow-arrow" :class="{ active: animationStep >= 2 }">
                            <svg
                              width="20"
                              height="20"
                              viewBox="0 0 24 24"
                              fill="none"
                              stroke="currentColor"
                              stroke-width="2"
                            >
                              <polyline points="9 18 15 12 9 6" />
                            </svg>
                          </div>
                          <div
                            class="workflow-step"
                            :class="{ active: animationStep >= 2, done: animationStep > 2 }"
                          >
                            <div class="step-icon">
                              <PhFunnel :size="24" />
                            </div>
                            <div class="step-label">Filter</div>
                          </div>
                          <div class="workflow-arrow" :class="{ active: animationStep >= 3 }">
                            <svg
                              width="20"
                              height="20"
                              viewBox="0 0 24 24"
                              fill="none"
                              stroke="currentColor"
                              stroke-width="2"
                            >
                              <polyline points="9 18 15 12 9 6" />
                            </svg>
                          </div>
                          <div
                            class="workflow-step"
                            :class="{ active: animationStep >= 3, done: animationStep > 3 }"
                          >
                            <div class="step-icon">
                              <PhShootingStar :size="24" />
                            </div>
                            <div class="step-label">Auto Star</div>
                          </div>
                          <div class="workflow-arrow" :class="{ active: animationStep >= 4 }">
                            <svg
                              width="20"
                              height="20"
                              viewBox="0 0 24 24"
                              fill="none"
                              stroke="currentColor"
                              stroke-width="2"
                            >
                              <polyline points="9 18 15 12 9 6" />
                            </svg>
                          </div>
                          <div
                            class="workflow-step success"
                            :class="{ active: animationStep >= 4, done: animationStep === 4 }"
                          >
                            <div class="step-icon">
                              <PhCheck :size="24" />
                            </div>
                            <div class="step-label">Done</div>
                          </div>
                        </div>
                        <div class="workflow-info" v-if="animationStep > 0 && animationStep < 4">
                          <div class="info-badge">Processing step {{ animationStep }} of 4</div>
                        </div>
                        <button class="demo-btn" @click="toggleWorkflow">
                          <span v-if="!isAnimating">Run Workflow</span>
                          <span v-else>Reset</span>
                        </button>
                      </div>

                      <!-- Feature 3: 100% Local Privacy -->
                      <div v-if="activeFeatureIndex === 3" class="demo-content privacy-demo">
                        <div class="demo-header">
                          <div class="demo-badge">Local Privacy</div>
                          <div class="demo-status" :class="{ active: isAnimating }">
                            <span class="status-dot"></span>
                            {{
                              privacyVerified
                                ? 'Verified ✓'
                                : isAnimating
                                  ? 'Verifying...'
                                  : 'Ready'
                            }}
                          </div>
                        </div>
                        <div class="privacy-container">
                          <div
                            class="privacy-shield"
                            :class="{ active: isAnimating, verified: privacyVerified }"
                          >
                            <div class="shield-rings">
                              <div class="ring ring-1"></div>
                              <div class="ring ring-2"></div>
                              <div class="ring ring-3"></div>
                            </div>
                            <div class="shield-icon">
                              <PhFingerprint :size="48" />
                            </div>
                            <div class="shield-text">
                              {{ privacyVerified ? '100% Secure' : '100% Local' }}
                            </div>
                          </div>
                          <div class="privacy-features">
                            <TransitionGroup name="feature">
                              <div
                                v-if="privacyVerified"
                                key="no-cloud"
                                class="privacy-feature"
                                :style="{ transitionDelay: '0ms' }"
                              >
                                <div class="privacy-icon">
                                  <PhCloudSlash :size="24" weight="bold" />
                                </div>
                                <div class="privacy-label">No Cloud</div>
                              </div>
                              <div
                                v-if="privacyVerified"
                                key="offline-ai"
                                class="privacy-feature"
                                :style="{ transitionDelay: '150ms' }"
                              >
                                <div class="privacy-icon">
                                  <PhCpu :size="24" weight="bold" />
                                </div>
                                <div class="privacy-label">Offline AI</div>
                              </div>
                            </TransitionGroup>
                          </div>
                        </div>
                        <button class="demo-btn" @click="togglePrivacy">
                          <span v-if="!isAnimating">Verify Privacy</span>
                          <span v-else>Reset</span>
                        </button>
                      </div>
                    </div>
                  </div>
                </div>
              </div>
            </div>
          </div>

          <div class="features-cards-sticky" :class="{ 'stacked-cards': isMobileOrSmallScreen }">
            <!-- Single card mode for desktop -->
            <div
              v-if="!isMobileOrSmallScreen"
              class="function-card"
              :class="[features[activeFeatureIndex].colorClass]"
            >
              <div class="card-glow"></div>
              <div class="card-pattern"></div>

              <div class="card-icon">
                <component :is="features[activeFeatureIndex].icon" />
              </div>

              <h3 class="card-title">{{ features[activeFeatureIndex].title }}</h3>

              <p class="card-description">{{ features[activeFeatureIndex].description }}</p>

              <ul class="card-features">
                <li v-for="(item, i) in features[activeFeatureIndex].highlights" :key="i">
                  <PhCheck :size="16" weight="bold" />
                  {{ item }}
                </li>
              </ul>
            </div>

            <!-- All cards stacked for mobile/small screens -->
            <template v-else>
              <div
                v-for="(feature, index) in features"
                :key="index"
                class="function-card stacked"
                :class="[feature.colorClass]"
              >
                <div class="card-glow"></div>
                <div class="card-pattern"></div>

                <div class="card-icon">
                  <component :is="feature.icon" />
                </div>

                <h3 class="card-title">{{ feature.title }}</h3>

                <p class="card-description">{{ feature.description }}</p>

                <ul class="card-features">
                  <li v-for="(item, i) in feature.highlights" :key="i">
                    <PhCheck :size="16" weight="bold" />
                    {{ item }}
                  </li>
                </ul>
              </div>
            </template>
          </div>
        </div>
      </div>
    </div>
  </section>
</template>

<style scoped>
.features-section {
  position: relative;
  padding: 0 2rem 4rem;
  background: #0d1117;
  color: #f0f6fc;
}

.section-background {
  display: none;
}

.container {
  position: relative;
  max-width: 1400px;
  margin: 0 auto;
  z-index: 1;
}

.section-header-sticky {
  position: sticky;
  top: 5rem;
  z-index: 100;
  padding: 3rem 0 2rem;
  display: flex;
  flex-direction: column;
  align-items: center;
  transition: all 0.3s ease;
}

.section-header-sticky.no-sticky {
  position: relative;
  top: 0;
}

.section-header-sticky.has-background {
  background: linear-gradient(to bottom, rgba(13, 17, 23, 0.95) 80%, rgba(13, 17, 23, 0) 100%);
  border-radius: 20px;
  padding: 1.5rem 2rem;
  border: none;
}

.section-header {
  text-align: center;
  margin-bottom: 0;
  position: relative;
}

.section-title {
  font-family: 'Space Grotesk', sans-serif;
  font-size: clamp(3rem, 4vw, 4rem);
  font-weight: 700;
  line-height: 1;
  margin-bottom: 1rem;
}

.title-outline {
  -webkit-text-stroke: 2px rgba(58, 146, 255, 0.5);
  color: transparent;
  display: block;
}

.title-fill {
  display: block;
  background: linear-gradient(135deg, #58a6ff, #1f6feb);
  font-weight: 900;
  -webkit-background-clip: text;
  -webkit-text-fill-color: transparent;
  background-clip: text;
}

.section-description {
  font-size: 1.125rem;
  color: rgba(240, 246, 252, 0.7);
  max-width: 600px;
  margin: 0 auto;
  line-height: 1.6;
}

.features-scroll-area {
  position: relative;
  padding-bottom: 0;
  height: 400vh;
}

.features-scroll-area.compact-mode {
  height: auto;
  padding-bottom: 4rem;
}

.features-layout {
  display: grid;
  grid-template-columns: 1.2fr 1fr;
  gap: 3rem;
  position: relative;
  align-items: start;
  height: 100%;
}

.features-layout.stacked-layout {
  grid-template-columns: 1fr;
  height: auto;
  gap: 2rem;
}

.feature-visual-sticky {
  position: sticky;
  top: 35vh;
  height: 600px;
  display: flex;
  align-items: center;
  align-self: start;
}

.feature-visual-large {
  width: 100%;
  height: 600px;
}

.visual-container {
  width: 100%;
  height: 100%;
  display: flex;
  align-items: center;
  justify-content: center;
}

.app-interface {
  width: 100%;
  height: 100%;
  background: #0d1117;
  border: 1px solid rgba(240, 246, 252, 0.1);
  border-radius: 20px;
  overflow: hidden;
  box-shadow: 0 20px 60px rgba(0, 0, 0, 0.4);
}

.interface-header {
  display: flex;
  align-items: center;
  gap: 1rem;
  padding: 1rem 1.5rem;
  background: linear-gradient(135deg, rgba(88, 166, 255, 0.08), rgba(31, 111, 235, 0.03));
  border-bottom: 1px solid rgba(240, 246, 252, 0.08);
}

.interface-dots {
  display: flex;
  gap: 0.5rem;
}

.interface-dots span {
  width: 12px;
  height: 12px;
  border-radius: 50%;
  background: rgba(0, 0, 0, 0.1);
}

.interface-dots span:nth-child(1) {
  background: #ff5f56;
}

.interface-dots span:nth-child(2) {
  background: #ffbd2e;
}

.interface-dots span:nth-child(3) {
  background: #27c93f;
}

.interface-title {
  font-family: 'Space Grotesk', sans-serif;
  font-weight: 600;
  font-size: 0.875rem;
  color: #f0f6fc;
}

.interface-body {
  display: flex;
  height: calc(600px - 57px);
}

.sidebar {
  width: 80px;
  background: rgba(240, 246, 252, 0.02);
  border-right: 1px solid rgba(240, 246, 252, 0.08);
  padding: 1.5rem 0.5rem;
  display: flex;
  flex-direction: column;
  gap: 0.75rem;
}

.sidebar-item {
  width: 100%;
  height: 48px;
  background: rgba(240, 246, 252, 0.04);
  border: none;
  border-radius: 12px;
  display: flex;
  align-items: center;
  justify-content: center;
  transition: all 0.3s;
  cursor: pointer;
  padding: 0;
}

.sidebar-item:hover {
  background: rgba(240, 246, 252, 0.08);
  transform: scale(1.05);
}

.sidebar-item.active {
  background: linear-gradient(135deg, rgba(88, 166, 255, 0.2), rgba(31, 111, 235, 0.1));
}

.sidebar-icon {
  width: 24px;
  height: 24px;
  border-radius: 6px;
  background: rgba(240, 246, 252, 0.1);
}

.main-content {
  flex: 1;
  padding: 2rem;
  display: flex;
  align-items: center;
  justify-content: center;
  height: 543px;
}

/* Demo Content Common Styles */
.demo-content {
  width: 100%;
  max-width: 400px;
  height: 100%;
  background: linear-gradient(135deg, rgba(88, 166, 255, 0.05), rgba(31, 111, 235, 0.02));
  border: 1px solid rgba(88, 166, 255, 0.15);
  border-radius: 16px;
  padding: 1.5rem;
  transition: all 0.3s ease;
  display: flex;
  flex-direction: column;
}

.demo-header {
  margin-bottom: 1rem;
  display: flex;
  justify-content: space-between;
  align-items: center;
  gap: 0.5rem;
}

.demo-badge {
  display: inline-block;
  padding: 0.375rem 0.875rem;
  background: linear-gradient(135deg, #58a6ff, #1f6feb);
  color: #0d1117;
  border-radius: 20px;
  font-size: 0.75rem;
  font-weight: 600;
}

.demo-status {
  display: flex;
  align-items: center;
  gap: 0.375rem;
  font-size: 0.75rem;
  color: rgba(240, 246, 252, 0.5);
  padding: 0.25rem 0.625rem;
  background: rgba(240, 246, 252, 0.04);
  border-radius: 12px;
  transition: all 0.3s ease;
}

.demo-status.active {
  color: #58a6ff;
  background: rgba(88, 166, 255, 0.12);
}

.status-dot {
  width: 6px;
  height: 6px;
  border-radius: 50%;
  background: rgba(240, 246, 252, 0.3);
  transition: all 0.3s ease;
}

.demo-status.active .status-dot {
  background: #58a6ff;
  animation: statusPulse 1.5s ease-in-out infinite;
}

@keyframes statusPulse {
  0%,
  100% {
    opacity: 1;
  }
  50% {
    opacity: 0.4;
  }
}

.demo-btn {
  margin-top: auto;
  padding: 0.625rem 1.25rem;
  background: linear-gradient(135deg, #58a6ff, #1f6feb);
  color: #0d1117;
  border: none;
  border-radius: 10px;
  font-size: 0.8125rem;
  font-weight: 600;
  cursor: pointer;
  display: flex;
  align-items: center;
  justify-content: center;
  gap: 0.5rem;
  transition: all 0.3s ease;
  position: relative;
  overflow: hidden;
  width: 100%;
  flex-shrink: 0;
}

.demo-btn::before {
  content: '';
  position: absolute;
  top: 50%;
  left: 50%;
  width: 0;
  height: 0;
  background: rgba(240, 246, 252, 0.2);
  border-radius: 50%;
  transform: translate(-50%, -50%);
  transition:
    width 0.6s ease,
    height 0.6s ease;
}

.demo-btn:hover:not(:disabled)::before {
  width: 300px;
  height: 300px;
}

.demo-btn:hover:not(:disabled) {
  transform: translateY(-2px);
  box-shadow: 0 8px 20px rgba(88, 166, 255, 0.4);
}

.demo-btn:disabled {
  opacity: 0.6;
  cursor: not-allowed;
}

/* AI Chat Demo */
.chat-container {
  display: flex;
  flex-direction: column;
  gap: 0.75rem;
  flex: 1;
  overflow-y: auto;
  min-height: 120px;
}

.chat-message {
  display: flex;
  gap: 0.5rem;
  animation: messageSlideIn 0.4s cubic-bezier(0.4, 0, 0.2, 1);
}

.chat-message.appear {
  animation: messageAppear 0.5s cubic-bezier(0.4, 0, 0.2, 1);
}

@keyframes messageSlideIn {
  from {
    opacity: 0;
    transform: translateY(10px);
  }
  to {
    opacity: 1;
    transform: translateY(0);
  }
}

@keyframes messageAppear {
  from {
    opacity: 0;
    transform: scale(0.95) translateY(5px);
  }
  to {
    opacity: 1;
    transform: scale(1) translateY(0);
  }
}

.message-avatar {
  width: 32px;
  height: 32px;
  border-radius: 50%;
  display: flex;
  align-items: center;
  justify-content: center;
  font-size: 0.75rem;
  font-weight: 700;
  flex-shrink: 0;
}

.user-message .message-avatar {
  background: rgba(240, 246, 252, 0.1);
  color: #f0f6fc;
}

.ai-message .message-avatar {
  background: linear-gradient(135deg, #58a6ff, #1f6feb);
  color: #0d1117;
}

.message-bubble {
  flex: 1;
  padding: 0.75rem 1rem;
  border-radius: 14px;
  font-size: 0.8125rem;
  line-height: 1.6;
}

.user-message .message-bubble {
  background: rgba(240, 246, 252, 0.08);
  color: #f0f6fc;
}

.ai-message .message-bubble {
  background: linear-gradient(135deg, rgba(58, 146, 255, 0.15), rgba(31, 111, 235, 0.08));
  color: #f0f6fc;
  border: 1px solid rgba(58, 146, 255, 0.2);
}

.summary-content {
  margin-bottom: 0.75rem;
  color: rgba(240, 246, 252, 0.9);
  line-height: 1.7;
}

.summary-stats {
  display: flex;
  gap: 0.75rem;
  margin-top: 0.75rem;
  padding-top: 0.75rem;
  border-top: 1px solid rgba(58, 146, 255, 0.2);
}

.stat-item {
  display: flex;
  align-items: center;
  gap: 0.375rem;
  font-size: 0.75rem;
  color: rgba(240, 246, 252, 0.7);
  font-weight: 500;
}

.stat-item svg {
  color: #58a6ff;
}

.typing-indicator {
  display: flex;
  gap: 0.375rem;
  padding: 0.5rem 0;
}

.typing-indicator span {
  width: 8px;
  height: 8px;
  border-radius: 50%;
  background: #58a6ff;
  animation: typingBounce 1.4s ease-in-out infinite;
}

.typing-indicator span:nth-child(1) {
  animation-delay: 0s;
}

.typing-indicator span:nth-child(2) {
  animation-delay: 0.2s;
}

.typing-indicator span:nth-child(3) {
  animation-delay: 0.4s;
}

@keyframes typingBounce {
  0%,
  60%,
  100% {
    transform: translateY(0);
    opacity: 0.4;
  }
  30% {
    transform: translateY(-12px);
    opacity: 1;
  }
}

/* Feed Discovery Demo */
.discovery-container {
  display: flex;
  flex-direction: column;
  gap: 1rem;
  flex: 1;
  overflow-y: auto;
  min-height: 120px;
}

.search-box {
  position: relative;
  display: flex;
  align-items: center;
  gap: 0.75rem;
  padding: 0.875rem 1rem;
  background: rgba(13, 17, 23, 0.6);
  border: 1px solid rgba(240, 246, 252, 0.1);
  border-radius: 12px;
  transition: all 0.3s ease;
}

.search-box.searching {
  border-color: rgba(58, 146, 255, 0.3);
  box-shadow: 0 0 0 3px rgba(58, 146, 255, 0.1);
}

.search-icon {
  font-size: 1.25rem;
  color: rgba(240, 246, 252, 0.6);
}

.search-input {
  flex: 1;
  font-size: 0.875rem;
  color: rgba(240, 246, 252, 0.8);
  font-weight: 500;
}

.search-pulse {
  position: absolute;
  right: 1rem;
  width: 20px;
  height: 20px;
}

.search-pulse::before,
.search-pulse::after {
  content: '';
  position: absolute;
  top: 50%;
  left: 50%;
  width: 100%;
  height: 100%;
  border-radius: 50%;
  background: rgba(58, 146, 255, 0.4);
  transform: translate(-50%, -50%);
  animation: searchPulse 1.5s ease-out infinite;
}

.search-pulse::after {
  animation-delay: 0.5s;
}

@keyframes searchPulse {
  0% {
    transform: translate(-50%, -50%) scale(0.8);
    opacity: 1;
  }
  100% {
    transform: translate(-50%, -50%) scale(2.5);
    opacity: 0;
  }
}

.feed-results {
  display: flex;
  flex-direction: column;
  gap: 0.625rem;
  min-height: 100px;
}

.feed-item {
  display: flex;
  align-items: center;
  gap: 0.75rem;
  padding: 0.875rem;
  background: rgba(22, 27, 34, 0.6);
  border: 1px solid rgba(240, 246, 252, 0.08);
  border-radius: 12px;
  transition: all 0.3s cubic-bezier(0.4, 0, 0.2, 1);
}

.feed-item:hover {
  background: rgba(58, 146, 255, 0.08);
  border-color: rgba(58, 146, 255, 0.3);
  transform: translateX(4px);
  box-shadow: 0 2px 8px rgba(58, 146, 255, 0.15);
}

.feed-enter-active {
  animation: feedSlideIn 0.5s cubic-bezier(0.4, 0, 0.2, 1);
}

.feed-leave-active {
  transition: all 0.3s ease;
}

.feed-leave-to {
  opacity: 0;
  transform: translateX(-20px);
}

@keyframes feedSlideIn {
  from {
    opacity: 0;
    transform: translateX(-20px) scale(0.95);
  }
  to {
    opacity: 1;
    transform: translateX(0) scale(1);
  }
}

.feed-icon {
  width: 40px;
  height: 40px;
  display: flex;
  align-items: center;
  justify-content: center;
  background: linear-gradient(135deg, #ffc107, #ffa000);
  color: white;
  border-radius: 10px;
  font-size: 0.625rem;
  font-weight: 700;
  flex-shrink: 0;
}

.feed-info {
  flex: 1;
  min-width: 0;
}

.feed-name {
  font-size: 0.875rem;
  font-weight: 600;
  color: #f0f6fc;
  margin-bottom: 0.25rem;
}

.feed-count {
  font-size: 0.75rem;
  color: rgba(240, 246, 252, 0.5);
}

.feed-add {
  width: 32px;
  height: 32px;
  display: flex;
  align-items: center;
  justify-content: center;
  background: linear-gradient(135deg, #58a6ff, #1f6feb);
  color: #0d1117;
  border: none;
  border-radius: 8px;
  cursor: pointer;
  transition: all 0.3s cubic-bezier(0.4, 0, 0.2, 1);
  flex-shrink: 0;
}

.feed-add:hover {
  box-shadow: 0 4px 12px rgba(88, 166, 255, 0.4);
}

/* Automation Workflow Demo */
.workflow-container {
  display: flex;
  align-items: center;
  justify-content: space-between;
  flex: 0;
  padding: 1rem 0;
  position: relative;
}

.workflow-step {
  display: flex;
  flex-direction: column;
  align-items: center;
  gap: 0.625rem;
  transition: all 0.4s cubic-bezier(0.4, 0, 0.2, 1);
  z-index: 1;
}

.workflow-step .step-icon {
  width: 56px;
  height: 56px;
  display: flex;
  align-items: center;
  justify-content: center;
  background: rgba(240, 246, 252, 0.05);
  border: 2px solid rgba(240, 246, 252, 0.1);
  border-radius: 14px;
  transition: all 0.4s cubic-bezier(0.4, 0, 0.2, 1);
  color: rgba(240, 246, 252, 0.4);
}

.workflow-step.active .step-icon {
  background: linear-gradient(135deg, #58a6ff, #1f6feb);
  border-color: #58a6ff;
  color: #0d1117;
  box-shadow: 0 4px 12px rgba(88, 166, 255, 0.3);
  transform: scale(1.05);
}

.workflow-step.done .step-icon {
  background: linear-gradient(135deg, rgba(46, 160, 67, 0.2), rgba(46, 160, 67, 0.1));
  border-color: #2ea043;
  color: #2ea043;
}

.workflow-step.success.active .step-icon {
  background: linear-gradient(135deg, #2ea043, #238636);
  border-color: #2ea043;
  color: #0d1117;
  animation: successPulse 0.6s cubic-bezier(0.4, 0, 0.2, 1);
}

@keyframes successPulse {
  0%,
  100% {
    transform: scale(1);
  }
  50% {
    transform: scale(1.2);
  }
}

.step-label {
  font-size: 0.6875rem;
  font-weight: 600;
  color: rgba(240, 246, 252, 0.5);
  text-align: center;
  transition: all 0.3s ease;
}

.workflow-step.active .step-label {
  color: #58a6ff;
}

.workflow-step.done .step-label {
  color: #2ea043;
}

.workflow-arrow {
  display: flex;
  align-items: center;
  color: rgba(240, 246, 252, 0.15);
  transition: all 0.4s cubic-bezier(0.4, 0, 0.2, 1);
  align-self: flex-start;
  margin-top: 20px;
}

.workflow-arrow.active {
  color: #58a6ff;
  animation: arrowFlow 1s cubic-bezier(0.4, 0, 0.2, 1);
}

@keyframes arrowFlow {
  0% {
    transform: translateX(-3px);
    opacity: 0.5;
  }
  50% {
    transform: translateX(3px);
    opacity: 1;
  }
  100% {
    transform: translateX(-3px);
    opacity: 0.5;
  }
}

.workflow-info {
  text-align: center;
  flex: 0;
  margin-top: 0.5rem;
}

.info-badge {
  display: inline-block;
  padding: 0.375rem 0.875rem;
  background: linear-gradient(135deg, rgba(88, 166, 255, 0.15), rgba(88, 166, 255, 0.08));
  border: 1px solid rgba(88, 166, 255, 0.3);
  border-radius: 20px;
  font-size: 0.75rem;
  font-weight: 600;
  color: #58a6ff;
}

/* Privacy Demo */
.privacy-container {
  display: flex;
  flex-direction: column;
  gap: 1.25rem;
  flex: 1;
  overflow-y: auto;
  min-height: 120px;
}

.privacy-shield {
  position: relative;
  display: flex;
  flex-direction: column;
  align-items: center;
  gap: 1rem;
  padding: 2rem 1.5rem;
  background: rgba(46, 160, 67, 0.06);
  border: 2px solid rgba(46, 160, 67, 0.2);
  border-radius: 20px;
  transition: all 0.5s cubic-bezier(0.4, 0, 0.2, 1);
}

.privacy-shield.active {
  background: linear-gradient(135deg, rgba(46, 160, 67, 0.12), rgba(46, 160, 67, 0.06));
  border-color: #2ea043;
}

.privacy-shield.verified {
  background: linear-gradient(135deg, rgba(46, 160, 67, 0.15), rgba(46, 160, 67, 0.08));
  border-color: #2ea043;
  box-shadow: 0 8px 24px rgba(46, 160, 67, 0.25);
}

.shield-rings {
  position: absolute;
  top: 50%;
  left: 50%;
  transform: translate(-50%, -50%);
  width: 120px;
  height: 120px;
  pointer-events: none;
}

.ring {
  position: absolute;
  top: 50%;
  left: 50%;
  transform: translate(-50%, -50%);
  border: 2px solid rgba(46, 160, 67, 0.4);
  border-radius: 50%;
  opacity: 0;
}

.privacy-shield.active .ring {
  animation: ringExpand 2s ease-out infinite;
}

.privacy-shield.active .ring-1 {
  width: 60px;
  height: 60px;
  animation-delay: 0s;
}

.privacy-shield.active .ring-2 {
  width: 90px;
  height: 90px;
  animation-delay: 0.4s;
}

.privacy-shield.active .ring-3 {
  width: 120px;
  height: 120px;
  animation-delay: 0.8s;
}

@keyframes ringExpand {
  0% {
    transform: translate(-50%, -50%) scale(0.5);
    opacity: 0.8;
  }
  100% {
    transform: translate(-50%, -50%) scale(1.5);
    opacity: 0;
  }
}

.shield-icon {
  position: relative;
  z-index: 1;
  color: rgba(46, 160, 67, 0.4);
  transition: all 0.5s cubic-bezier(0.4, 0, 0.2, 1);
}

.privacy-shield.active .shield-icon {
  color: #2ea043;
  animation: shieldScale 0.6s cubic-bezier(0.4, 0, 0.2, 1);
}

@keyframes shieldScale {
  0%,
  100% {
    transform: scale(1);
  }
  50% {
    transform: scale(1.15);
  }
}

.shield-text {
  position: relative;
  z-index: 1;
  font-size: 1rem;
  font-weight: 700;
  color: #2ea043;
  transition: all 0.3s ease;
}

.privacy-features {
  display: flex;
  flex-direction: row;
  gap: 1rem;
  min-height: 100px;
}

.privacy-feature {
  flex: 1;
  display: flex;
  flex-direction: column;
  align-items: center;
  justify-content: center;
  gap: 0.75rem;
  padding: 1.25rem 1rem;
  background: rgba(22, 27, 34, 0.6);
  border: 1px solid rgba(240, 246, 252, 0.08);
  border-radius: 12px;
  transition: all 0.3s cubic-bezier(0.4, 0, 0.2, 1);
}

.privacy-feature:hover {
  border-color: rgba(46, 160, 67, 0.3);
  box-shadow: 0 2px 8px rgba(46, 160, 67, 0.15);
}

.feature-enter-active {
  animation: featureSlideIn 0.5s cubic-bezier(0.4, 0, 0.2, 1);
}

.feature-leave-active {
  transition: all 0.3s ease;
}

.feature-leave-to {
  opacity: 0;
  transform: translateX(-20px);
}

@keyframes featureSlideIn {
  from {
    opacity: 0;
    transform: translateX(-20px) scale(0.95);
  }
  to {
    opacity: 1;
    transform: translateX(0) scale(1);
  }
}

.privacy-icon {
  width: 48px;
  height: 48px;
  display: flex;
  align-items: center;
  justify-content: center;
  background: linear-gradient(135deg, rgba(46, 160, 67, 0.2), rgba(46, 160, 67, 0.1));
  border: 2px solid rgba(46, 160, 67, 0.3);
  border-radius: 12px;
  flex-shrink: 0;
  color: #2ea043;
}

.privacy-label {
  font-size: 0.875rem;
  font-weight: 600;
  color: #f0f6fc;
}

.features-cards-sticky {
  position: sticky;
  top: 35vh;
  height: 600px;
  display: flex;
  align-items: center;
  align-self: start;
}

.features-cards-sticky.stacked-cards {
  position: relative;
  top: 0;
  height: auto;
  display: flex;
  flex-direction: column;
  gap: 2rem;
}

.function-card {
  position: relative;
  padding: 0;
  background: transparent;
  border: none;
  border-radius: 0;
  overflow: visible;
  height: 600px;
  transition: none;
  display: flex;
  flex-direction: column;
  justify-content: center;
  width: 100%;
}

.function-card.stacked {
  height: auto;
  padding: 2.5rem;
  background: #0d1117;
  border: 1px solid rgba(240, 246, 252, 0.1);
  border-radius: 20px;
  transition: all 0.3s ease;
  width: 100%;
  max-width: 600px;
  margin: 0 auto;
}

.function-card.stacked:hover {
  border-color: rgba(88, 166, 255, 0.3);
  box-shadow: 0 8px 30px rgba(0, 0, 0, 0.3);
  transform: translateY(-4px);
}

.function-card::before {
  display: none;
}

.function-card:hover {
  transform: none;
  box-shadow: none;
  border-color: transparent;
}

.function-card:hover .card-glow {
  opacity: 0;
}

.function-card:hover .card-pattern {
  opacity: 0;
}

.card-icon {
  position: relative;
  width: 80px;
  height: 80px;
  display: flex;
  align-items: center;
  justify-content: center;
  margin-bottom: 1.5rem;
  z-index: 1;
  transition: none;
}

.function-card:hover .card-icon {
  transform: none;
}

.function-card.blue .card-icon {
  background: linear-gradient(135deg, rgba(88, 166, 255, 0.15), rgba(31, 111, 235, 0.08));
  border: 1px solid rgba(88, 166, 255, 0.3);
  border-radius: 20px;
  color: #58a6ff;
}

.function-card.purple .card-icon {
  background: linear-gradient(135deg, rgba(255, 193, 7, 0.15), rgba(255, 152, 0, 0.08));
  border: 1px solid rgba(255, 193, 7, 0.3);
  border-radius: 20px;
  color: #ffc107;
}

.function-card.yellow .card-icon {
  background: linear-gradient(135deg, rgba(88, 166, 255, 0.15), rgba(31, 111, 235, 0.08));
  border: 1px solid rgba(88, 166, 255, 0.3);
  border-radius: 20px;
  color: #58a6ff;
}

.function-card.green .card-icon {
  background: linear-gradient(135deg, rgba(46, 160, 67, 0.15), rgba(46, 160, 67, 0.08));
  border: 1px solid rgba(46, 160, 67, 0.3);
  border-radius: 20px;
  color: #2ea043;
}

.card-title {
  position: relative;
  font-family: 'Space Grotesk', sans-serif;
  font-size: 1.75rem;
  font-weight: 700;
  color: #f0f6fc;
  margin-bottom: 1rem;
  z-index: 1;
}

.card-description {
  position: relative;
  font-size: 1.0625rem;
  color: rgba(240, 246, 252, 0.7);
  line-height: 1.7;
  margin-bottom: 1.5rem;
  z-index: 1;
}

.card-features {
  position: relative;
  display: flex;
  flex-direction: column;
  gap: 0.75rem;
  margin-bottom: 1.5rem;
  z-index: 1;
}

.card-features li {
  display: flex;
  align-items: center;
  gap: 0.75rem;
  font-size: 0.9375rem;
  color: rgba(240, 246, 252, 0.75);
}

.card-features li svg {
  flex-shrink: 0;
  opacity: 0.8;
}

.function-card.blue .card-features li svg {
  color: #58a6ff;
}

.function-card.purple .card-features li svg {
  color: #ffc107;
}

.function-card.yellow .card-features li svg {
  color: #58a6ff;
}

.function-card.green .card-features li svg {
  color: #2ea043;
}

.function-card::after {
  display: none;
}

@media (max-width: 1024px) {
  .scroll-indicator {
    display: none;
  }

  .features-section {
    padding: 0 1rem 4rem;
  }

  .section-header-sticky.no-sticky {
    position: relative;
    top: 0;
    padding: 3rem 0 2rem;
  }

  .features-scroll-area {
    height: auto;
    padding: 2rem 0;
  }

  .features-layout {
    grid-template-columns: 1fr;
    gap: 2rem;
    height: auto;
  }

  .feature-visual-sticky {
    position: relative;
    top: 0;
    height: 600px;
  }

  .feature-visual-large {
    height: 600px;
    max-height: 600px;
  }

  .features-cards-sticky {
    position: relative;
    top: 0;
    height: 600px;
  }

  .function-card {
    height: 600px;
  }
}

@media (max-width: 768px) {
  .section-header-sticky.no-sticky {
    padding: 2rem 0 1rem;
  }

  .section-header {
    margin-bottom: 1.5rem;
  }

  .section-title {
    font-size: 2rem;
  }

  .section-description {
    font-size: 1rem;
  }

  /* Hide visual animations on mobile */
  .feature-visual-sticky {
    display: none;
  }

  /* Cards stack vertically with full styling */
  .features-cards-sticky.stacked-cards {
    gap: 1.5rem;
  }

  .function-card.stacked {
    padding: 1.75rem;
  }

  .card-title {
    font-size: 1.375rem;
  }

  .card-description {
    font-size: 0.9375rem;
  }

  .card-features li {
    font-size: 0.875rem;
  }
}

@media (max-width: 480px) {
  .features-section {
    padding: 0 0.75rem 3rem;
  }

  .function-card.stacked {
    padding: 1.5rem;
  }

  .card-icon {
    width: 64px;
    height: 64px;
  }

  .card-title {
    font-size: 1.25rem;
  }

  .card-description {
    font-size: 0.875rem;
  }
}

/* Handle small height screens (e.g., laptops with 768px height) */
@media (max-height: 700px) {
  .section-header-sticky.no-sticky {
    position: relative;
    top: 0;
    padding: 2rem 0 1.5rem;
  }

  .features-scroll-area {
    padding-bottom: 3rem;
  }

  .feature-visual-sticky {
    top: 20vh;
    height: 450px;
  }

  .feature-visual-large {
    height: 450px;
  }

  .features-cards-sticky {
    top: 20vh;
    height: 450px;
  }

  .function-card {
    height: 450px;
  }

  .interface-body {
    height: calc(450px - 57px);
  }

  .main-content {
    height: 393px;
  }
}
</style>
