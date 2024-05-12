package backend

import (
	"encoding/json"
	"log"
	"os"
)

type FeedInfo struct {
	Title string
	Link  string
}

func readFeedFromFile(filename string) []FeedInfo {
	file, err := os.ReadFile(filename)
	if err != nil {
		log.Fatalf("Unable to read file: %v", err)
	}

	var feeds []FeedInfo
	err = json.Unmarshal(file, &feeds)
	if err != nil {
		log.Fatalf("Unable to parse JSON: %v", err)
	}

	return feeds
}

func GetFeedList() []FeedInfo {
	feedList := readFeedFromFile("data/feeds.json")
	return feedList
}
