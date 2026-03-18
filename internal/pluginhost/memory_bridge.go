package pluginhost

import (
	"strings"
	"time"

	"kizuna/internal/memory"
	"kizuna/pkg/pluginapi"
)

func pluginMemoryMessageFromRecord(record memory.MessageRecord) pluginapi.MemoryMessage {
	message := pluginapi.MemoryMessage{
		Role: strings.TrimSpace(record.Role),
		Guild: pluginapi.GuildInfo{
			ID: strings.TrimSpace(record.GuildID),
		},
		Content: strings.TrimSpace(record.Content),
		Time:    record.Time.Format(time.RFC3339),
		Author: pluginapi.UserInfo{
			ID:          strings.TrimSpace(record.Author.UserID),
			Username:    strings.TrimSpace(record.Author.Username),
			GlobalName:  strings.TrimSpace(record.Author.GlobalName),
			Nick:        strings.TrimSpace(record.Author.Nick),
			DisplayName: strings.TrimSpace(record.Author.DisplayName),
		},
	}
	if record.ReplyTo != nil {
		message.ReplyTo = &pluginapi.ReplyInfo{
			MessageID: strings.TrimSpace(record.ReplyTo.MessageID),
			Role:      strings.TrimSpace(record.ReplyTo.Role),
			Content:   strings.TrimSpace(record.ReplyTo.Content),
			Time:      record.ReplyTo.Time.Format(time.RFC3339),
			Author: pluginapi.UserInfo{
				ID:          strings.TrimSpace(record.ReplyTo.Author.UserID),
				Username:    strings.TrimSpace(record.ReplyTo.Author.Username),
				GlobalName:  strings.TrimSpace(record.ReplyTo.Author.GlobalName),
				Nick:        strings.TrimSpace(record.ReplyTo.Author.Nick),
				DisplayName: strings.TrimSpace(record.ReplyTo.Author.DisplayName),
			},
		}
	}
	if len(record.Images) > 0 {
		message.Images = make([]pluginapi.ImageReference, 0, len(record.Images))
		for _, image := range record.Images {
			message.Images = append(message.Images, pluginapi.ImageReference{
				Kind:        strings.TrimSpace(image.Kind),
				Name:        strings.TrimSpace(image.Name),
				EmojiID:     strings.TrimSpace(image.EmojiID),
				URL:         strings.TrimSpace(image.URL),
				Animated:    image.Animated,
				ContentType: strings.TrimSpace(image.ContentType),
			})
		}
	}
	return message
}

func memoryRecordFromPluginMemoryMessage(message pluginapi.MemoryMessage) memory.MessageRecord {
	record := memory.MessageRecord{
		Role:    strings.TrimSpace(message.Role),
		GuildID: strings.TrimSpace(message.Guild.ID),
		Content: strings.TrimSpace(message.Content),
		Author: memory.MessageAuthor{
			UserID:      strings.TrimSpace(message.Author.ID),
			Username:    strings.TrimSpace(message.Author.Username),
			GlobalName:  strings.TrimSpace(message.Author.GlobalName),
			Nick:        strings.TrimSpace(message.Author.Nick),
			DisplayName: strings.TrimSpace(message.Author.DisplayName),
		},
	}
	if parsedTime, err := time.Parse(time.RFC3339, strings.TrimSpace(message.Time)); err == nil {
		record.Time = parsedTime
	}
	if message.ReplyTo != nil {
		record.ReplyTo = &memory.ReplyRecord{
			MessageID: strings.TrimSpace(message.ReplyTo.MessageID),
			Role:      strings.TrimSpace(message.ReplyTo.Role),
			Content:   strings.TrimSpace(message.ReplyTo.Content),
			Author: memory.MessageAuthor{
				UserID:      strings.TrimSpace(message.ReplyTo.Author.ID),
				Username:    strings.TrimSpace(message.ReplyTo.Author.Username),
				GlobalName:  strings.TrimSpace(message.ReplyTo.Author.GlobalName),
				Nick:        strings.TrimSpace(message.ReplyTo.Author.Nick),
				DisplayName: strings.TrimSpace(message.ReplyTo.Author.DisplayName),
			},
		}
		if parsedTime, err := time.Parse(time.RFC3339, strings.TrimSpace(message.ReplyTo.Time)); err == nil {
			record.ReplyTo.Time = parsedTime
		}
	}
	if len(message.Images) > 0 {
		record.Images = make([]memory.ImageReference, 0, len(message.Images))
		for _, image := range message.Images {
			record.Images = append(record.Images, memory.ImageReference{
				Kind:        strings.TrimSpace(image.Kind),
				Name:        strings.TrimSpace(image.Name),
				EmojiID:     strings.TrimSpace(image.EmojiID),
				URL:         strings.TrimSpace(image.URL),
				Animated:    image.Animated,
				ContentType: strings.TrimSpace(image.ContentType),
			})
		}
	}
	return record
}

func pluginMemorySearchResults(records []memory.VectorRecord) []pluginapi.MemorySearchResult {
	results := make([]pluginapi.MemorySearchResult, 0, len(records))
	for _, record := range records {
		results = append(results, pluginapi.MemorySearchResult{
			Content:  strings.TrimSpace(record.Content),
			Rendered: strings.TrimSpace(record.Rendered),
			Time:     record.Time.Format(time.RFC3339),
		})
	}
	return results
}
