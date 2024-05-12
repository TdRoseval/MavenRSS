package backend

import (
	"crypto/md5"
	"fmt"
)

type FeedContentFilterInfo struct {
	FeedTitle string
	FeedImage string
	Title     string
	Link      string
	TimeSince string
	Time      string
	Image     string
	Content   string
	Readed    bool
}

func FilterFeedContent() []FeedContentFilterInfo {
	history := GetHistory()

	feedContent := GetFeedContent()

	var feedContentInfo []FeedContentFilterInfo

	for _, item := range feedContent {
		// Get the image URL
		imageURL := ""
		filterImageUrl := FilterImage(item.Item.Content)
		if item.Item.Image != nil {
			imageURL = item.Item.Image.URL
		} else if filterImageUrl != nil {
			imageURL = *filterImageUrl
		}

		// Get the time since the item was published
		timeSinceStr := TimeSince(item.Item.PublishedParsed)

		// Check if the item is in the history
		readed := false
		hash := fmt.Sprintf("%x", md5.Sum([]byte(item.Item.Title+item.Item.Content)))
		if _, ok := history[hash]; ok {
			if history[hash].Readed {
				readed = true
			}
		}

		// Append the item to the feedContentInfo
		feedContentInfo = append(feedContentInfo, FeedContentFilterInfo{
			FeedTitle: item.Title,
			FeedImage: item.Image,
			Title:     item.Item.Title,
			Link:      item.Item.Link,
			TimeSince: timeSinceStr,
			Time:      item.Item.PublishedParsed.Format("2006-01-02 15:04"),
			Image:     imageURL,
			Content:   item.Item.Content,
			Readed:    readed,
		})
	}

	return feedContentInfo
}
