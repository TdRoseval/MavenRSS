<script lang="ts" setup>

const props = defineProps({
  feedContent: {
    type: Object,
    default: () => ({ feedList: [] })
  }
})
</script>

<template>
  <ul>
    <li v-for="feed in feedContent.feedList" :key="feed.Title" @click="$emit('feed-clicked', feed)">
      <button class="ListItem" title="Read more">
        <img v-if="feed.Image" :src="feed.Image" :alt="`Image of post: ${ feed.Title }`" class="Image" />
        <div class="Info">
          <div class="RssInfo">
            <div class="RssTitle">
              <img v-if="feed.FeedImage" :src="feed.FeedImage" :alt="`Image of feed: ${feed.FeedTitle}`"
                class="RssImage" />
              <p>{{ feed.FeedTitle }}</p>
            </div>
            <time>{{ feed.TimeSince }}</time>
          </div>
          <p class="Title" :style="{ color: feed.Readed ? 'gray' : 'black' }">{{ feed.Title }}</p>
        </div>
      </button>
    </li>
  </ul>
</template>

<style scoped>
ul {
  list-style-type: none;
  padding: 0;
  margin: 0;

  overflow-y: scroll;
}

ul::-webkit-scrollbar {
  width: 8px;
}
ul::-webkit-scrollbar-thumb {
  background-color: #ccc;
  cursor: pointer;
}
ul::-webkit-scrollbar-track {
  background-color: #f1f1f1;
}
ul::-webkit-scrollbar-thumb:hover {
  background-color: #999;
}

li {
  max-height: 104px;
}

.ListItem {
  display: flex;
  border: none;
  border-top: 0.5px solid #ccc;
  text-decoration: none;
  color: #333;
  background-color: #f9f9f9;
  width: 100%;
  height: 100%;
  padding: 8px;
  cursor: pointer;
}

.Image {
  object-fit: cover;
  min-width: 88px;
  max-width: 88px;
  min-height: 88px;
  max-height: 88px;
  margin-right: 8px;
}

.RssImage {
  object-fit: cover;
  min-width: 16px;
  max-width: 16px;
  min-height: 16px;
  max-height: 16px;
  margin-right: 4px;
  border-radius: 50%;
}

.ListItem .Info {
  display: flex;
  flex-direction: column;
  justify-content: start;

  width: 100%;
}

.ListItem .RssInfo {
  display: flex;
  justify-content: space-between;
  
  height: 16px;
  font-size: small;
}

.ListItem .RssTitle {
  display: flex;
  align-items: center;
}

.ListItem .Title {
  font-size: medium;
  font-weight: bold;
  width: 100%;
  height: 64px;
  text-align: left;
  padding-top: 8px;

  overflow: hidden;
  -webkit-line-clamp: 3;
  text-overflow: ellipsis;
  display: -webkit-box;
  -webkit-box-orient: vertical;
}

p, time {
  display: flex;
  align-items: center;
  height: 16px;
}
</style>
