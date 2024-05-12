package backend

import (
	"crypto/md5"
	"encoding/json"
	"fmt"
	"os"
)

func writeHistoryToFile(filename string, history []FeedContentFilterInfo) error {
	if history == nil {
		history = []FeedContentFilterInfo{}
	}

	historyMap := make(map[string]FeedContentFilterInfo)

	// Calculate the hash of the history
	for _, item := range history {
		hash := fmt.Sprintf("%x", md5.Sum([]byte(item.Title+item.Content)))
		historyMap[hash] = item
	}

	data, err := json.MarshalIndent(historyMap, "", "    ")
	if err != nil {
		return err
	}

	err = os.WriteFile(filename, data, 0644)
	if err != nil {
		return err
	}

	return nil
}

func WriteHistory(history []FeedContentFilterInfo) error {
	if history == nil {
		history = []FeedContentFilterInfo{}
	}

	err := writeHistoryToFile("data/history.json", history)
	if err != nil {
		return err
	}

	return nil
}
