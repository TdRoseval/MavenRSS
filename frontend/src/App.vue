<script lang="ts" setup>
import { defineComponent, reactive, ref, onMounted } from 'vue'
import { Icon } from '@iconify/vue';
import RssList from './components/RssList.vue'
import RssContent from './components/RssContent.vue'
import { GetFeedContent, GetHistoryContent, WriteHistory } from '../wailsjs/go/main/App'

type FeedContent = {
  FeedTitle: string
  FeedImage: string
  Title: string
  Link: string
  TimeSince: string
  Time: string
  Image: string
  Content: string
  Readed: boolean
}

const feedContent = reactive({
  feedList: [] as FeedContent[],
})

const selectedFeed = ref<FeedContent | null>(null)

async function fetchFeedContent() {
  const result: FeedContent[] = await GetFeedContent()
  feedContent.feedList = result
  return feedContent
}

async function fetchHistoryContent() {
  const result: FeedContent[] = await GetHistoryContent()
  feedContent.feedList = result
  return feedContent
}

async function setHistoryContent() {
  await WriteHistory(feedContent.feedList)
}

async function deleteHistoryContent() {
  feedContent.feedList = []
  await setHistoryContent()
  await handleClickRefresh()
}

const isRefreshing = ref(false)

async function handleClickRefresh() {
  isRefreshing.value = true
  await fetchHistoryContent()
  await fetchFeedContent()
  await setHistoryContent()
  isRefreshing.value = false
}

async function handleFeedClicked(feed: FeedContent) {
  selectedFeed.value = feed
  await modifyFeedContentReaded(feed, true)
}

async function modifyFeedContentReaded(feed: FeedContent, readed: boolean) {
  const index = feedContent.feedList.findIndex((f) => f.Title === feed.Title && f.Content === feed.Content)
  if (index !== -1) {
    if (feedContent.feedList[index].Readed !== readed) {
      feedContent.feedList[index].Readed = readed
      await setHistoryContent()
    }
  }
}

defineComponent({
  components: {
    feedContent,
  },
  setup(_, { emit }) {
    return {
      isRefreshing,
      handleClickRefresh,
      deleteHistoryContent,
      modifyFeedContentReaded,
    }
  }
})

onMounted(() => {
  handleClickRefresh()
})
</script>

<template>
  <aside>
    <nav>
      <div class="util">
        <button class="btn" title="Filter">
          <Icon icon="material-symbols:filter-list" />
        </button>
      </div>
      <div class="function">
        <button class="btn" @click="handleClickRefresh" title="Refresh">
          <Icon v-if="!isRefreshing" icon="material-symbols:refresh" />
          <Icon v-else icon="material-symbols:error" disabled="true" />
        </button>
        <button class="btn" title="Translate">
          <Icon icon="material-symbols:g-translate" />
        </button>
        <button class="btn" @click="deleteHistoryContent" title="Delete history">
          <Icon icon="material-symbols:auto-delete-outline" />
        </button>
        <button class="btn" title="Settings">
          <Icon icon="material-symbols:settings" />
        </button>
      </div>
    </nav>
    <rss-list @feed-clicked="handleFeedClicked" :feedContent="feedContent" />
  </aside>
  <main>
    <div class="ContentFunction">
      <button class="btn" title="Back to top" :disabled="!selectedFeed" :style="!selectedFeed ? { color: 'gray' } : {}">
        <Icon icon="material-symbols:vertical-align-top" />
      </button>
      <button class="btn" title="Bookmark" :disabled="!selectedFeed" :style="!selectedFeed ? { color: 'gray' } : {}">
        <Icon icon="material-symbols:collections-bookmark" />
      </button>
      <button class="btn" @click="selectedFeed && modifyFeedContentReaded(selectedFeed, false)" title="Unread" :disabled="!selectedFeed" :style="!selectedFeed ? { color: 'gray' } : {}">
        <Icon icon="material-symbols:thread-unread" />
      </button>
      <button class="btn" title="Open in browser" :disabled="!selectedFeed"
        :style="!selectedFeed ? { color: 'gray' } : {}">
        <Icon icon="material-symbols:open-in-browser" />
      </button>
      <button class="btn" title="Share" :disabled="!selectedFeed" :style="!selectedFeed ? { color: 'gray' } : {}">
        <Icon icon="material-symbols:share" />
      </button>
      <button class="btn" title="Translate" :disabled="!selectedFeed" :style="!selectedFeed ? { color: 'gray' } : {}">
        <Icon icon="material-symbols:g-translate" />
      </button>
      <button class="btn" title="Chat with AI" :disabled="!selectedFeed"
        :style="!selectedFeed ? { color: 'gray' } : {}">
        <Icon icon="material-symbols:robot-2" />
      </button>
    </div>
    <rss-content v-if="selectedFeed" :selectedFeed="selectedFeed" />
    <div v-else class="NoSelectedFeed"></div>
  </main>
</template>

<style>
#app {
  display: flex;
}

p {
  margin: 0;
}

aside {
  display: flex;
  flex-direction: column;

  min-width: 344px;
  max-width: 344px;
  height: 100vh;

  color: #000000;
  background-color: #f0f0f0;

  word-wrap: normal;
}

aside nav {
  position: sticky;
  top: 0;
  width: 100%;
  display: flex;
  justify-content: space-between;
  align-items: center;
  background-color: #e0e0e0;
}

aside .util, aside .function {
  display: flex;
  align-items: center;
}

aside .btn {
  width: 32px;
  height: 32px;
  font-size: large;
  background-color: #e0e0e0;
  border: none;
  cursor: pointer;

  display: flex;
  justify-content: center;
  align-items: center;
}

aside .btn:hover {
  background-color: #d0d0d0;
}

aside p {
  height: 32px;
  display: flex;
  align-items: center;
}

main {
  display: flex;
  flex-direction: column;

  width: calc(100vw - 344px);

  border-left: 1px solid #ccc;

  height: 100vh;
}

.ContentFunction {
  position: sticky;
  top: 0;
  width: 100%;
  display: flex;
  justify-content: end;
  align-items: center;
  background-color: #e0e0e0;
}

.ContentFunction .btn {
  width: 32px;
  height: 32px;
  font-size: large;
  background-color: #e0e0e0;
  border: none;
  cursor: pointer;

  display: flex;
  justify-content: center;
  align-items: center;
}

.ContentFunction .btn:hover {
  background-color: #d0d0d0;
}

.NoSelectedFeed {
  width: 100%;
  height: 100%;
  background-color: #f0f0f0;
}
</style>