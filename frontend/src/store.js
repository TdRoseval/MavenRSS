import { reactive, computed } from 'vue'

export const store = reactive({
    articles: [],
    feeds: [],
    currentFilter: 'all', // 'all', 'unread', 'favorites'
    currentFeedId: null,
    currentCategory: null,
    currentArticleId: null,
    isLoading: false,
    page: 1,
    hasMore: true,
    searchQuery: '',
    theme: localStorage.getItem('theme') || 'light',
    
    // Actions
    setFilter(filter) {
        this.currentFilter = filter;
        this.currentFeedId = null;
        this.currentCategory = null;
        this.page = 1;
        this.articles = [];
        this.hasMore = true;
        this.fetchArticles();
    },
    
    setFeed(feedId) {
        this.currentFilter = '';
        this.currentFeedId = feedId;
        this.currentCategory = null;
        this.page = 1;
        this.articles = [];
        this.hasMore = true;
        this.fetchArticles();
    },
    
    setCategory(category) {
        this.currentFilter = '';
        this.currentFeedId = null;
        this.currentCategory = category;
        this.page = 1;
        this.articles = [];
        this.hasMore = true;
        this.fetchArticles();
    },

    async fetchArticles(append = false) {
        if (this.isLoading) return;
        if (!append && !this.hasMore) this.hasMore = true; // Reset if new search
        
        this.isLoading = true;
        const limit = 50;
        
        let url = `/api/articles?page=${this.page}&limit=${limit}`;
        if (this.currentFilter) url += `&filter=${this.currentFilter}`;
        if (this.currentFeedId) url += `&feed_id=${this.currentFeedId}`;
        if (this.currentCategory) url += `&category=${encodeURIComponent(this.currentCategory)}`;
        
        try {
            const res = await fetch(url);
            const data = await res.json();
            const newArticles = data || [];
            
            if (newArticles.length < limit) {
                this.hasMore = false;
            }
            
            if (append) {
                this.articles = [...this.articles, ...newArticles];
            } else {
                this.articles = newArticles;
            }
        } catch (e) {
            console.error(e);
        } finally {
            this.isLoading = false;
        }
    },

    async loadMore() {
        if (this.hasMore && !this.isLoading) {
            this.page++;
            await this.fetchArticles(true);
        }
    },

    async fetchFeeds() {
        try {
            const res = await fetch('/api/feeds');
            const data = await res.json();
            this.feeds = data || [];
        } catch (e) {
            console.error(e);
            this.feeds = [];
        }
    },

    toggleTheme() {
        this.theme = this.theme === 'light' ? 'dark' : 'light';
        localStorage.setItem('theme', this.theme);
        if (this.theme === 'dark') {
            document.body.classList.add('dark-mode');
        } else {
            document.body.classList.remove('dark-mode');
        }
    },

    // Auto Refresh
    refreshInterval: null,
    refreshProgress: { current: 0, total: 0, isRunning: false },

    async refreshFeeds() {
        this.refreshProgress.isRunning = true;
        try {
            await fetch('/api/refresh', { method: 'POST' });
            this.pollProgress();
        } catch (e) {
            console.error(e);
            this.refreshProgress.isRunning = false;
        }
    },

    pollProgress() {
        let lastCurrent = 0;
        const interval = setInterval(async () => {
            try {
                const res = await fetch('/api/progress');
                const data = await res.json();
                this.refreshProgress = {
                    current: data.current,
                    total: data.total,
                    isRunning: data.is_running
                };

                // Progressive refresh: update articles whenever progress advances
                if (data.current > lastCurrent) {
                    lastCurrent = data.current;
                    this.fetchArticles();
                }

                if (!data.is_running) {
                    clearInterval(interval);
                    this.fetchFeeds();
                    this.fetchArticles();
                }
            } catch (e) {
                clearInterval(interval);
                this.refreshProgress.isRunning = false;
            }
        }, 500);
    },

    startAutoRefresh(minutes) {
        if (this.refreshInterval) clearInterval(this.refreshInterval);
        if (minutes > 0) {
            this.refreshInterval = setInterval(() => {
                this.refreshFeeds();
            }, minutes * 60 * 1000);
        }
    }
});
