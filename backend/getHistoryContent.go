package backend

import (
	"sort"
)

func GetHistoryContent() []FeedContentFilterInfo {
	history := GetHistory()

	// Turn the map into a slice
	var historySlice []FeedContentFilterInfo
	for _, item := range history {
		historySlice = append(historySlice, item)
	}

	// Sort the slice by time
	sort.Slice(historySlice, func(i, j int) bool {
		return historySlice[i].Time > historySlice[j].Time
	})

	return historySlice
}
