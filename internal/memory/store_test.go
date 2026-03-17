package memory

import (
	"context"
	"strings"
	"testing"
	"time"
)

func TestAddMessageIndexesWithDetachedContext(t *testing.T) {
	done := make(chan struct{})

	store := NewStore(func(ctx context.Context, input string) ([]float64, error) {
		if err := ctx.Err(); err != nil {
			t.Fatalf("expected detached context, got %v", err)
		}
		close(done)
		return []float64{1, 2, 3}, nil
	})

	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	store.AddMessage(ctx, "channel-1", "user", "hello")

	select {
	case <-done:
	case <-time.After(2 * time.Second):
		t.Fatal("timed out waiting for background indexing")
	}
}

func TestMessageRecordRenderForModelIncludesMetadata(t *testing.T) {
	record := MessageRecord{
		Role:    "user",
		Content: "hello world",
		Time:    time.Date(2026, 3, 18, 12, 34, 56, 0, time.UTC),
		Author: MessageAuthor{
			UserID:      "user-1",
			Username:    "alice",
			GlobalName:  "Alice Global",
			Nick:        "Alice Nick",
			DisplayName: "Alice Nick",
		},
	}

	rendered := record.RenderForModel()
	checks := []string{
		"时间(UTC+8): 2026-03-18 20:34:56",
		"发送者ID: user-1",
		"发送者用户名: alice",
		"发送者全局名: Alice Global",
		"发送者频道昵称: Alice Nick",
		"发送者显示名: Alice Nick",
		"内容:\nhello world",
	}
	for _, check := range checks {
		if !strings.Contains(rendered, check) {
			t.Fatalf("expected %q in rendered message, got %q", check, rendered)
		}
	}
}

func TestMessageRecordRenderForModelIncludesReplyContext(t *testing.T) {
	record := MessageRecord{
		Role:    "user",
		Content: "follow up",
		Time:    time.Date(2026, 3, 18, 12, 34, 56, 0, time.UTC),
		Author: MessageAuthor{
			UserID:      "user-1",
			Username:    "alice",
			DisplayName: "Alice",
		},
		ReplyTo: &ReplyRecord{
			MessageID: "msg-0",
			Role:      "assistant",
			Content:   "earlier answer",
			Time:      time.Date(2026, 3, 18, 12, 30, 0, 0, time.UTC),
			Author: MessageAuthor{
				UserID:      "bot-1",
				Username:    "helperbot",
				DisplayName: "Helper Bot",
			},
		},
	}

	rendered := record.RenderForModel()
	checks := []string{
		"这条消息是在回复以下消息:",
		"被回复消息ID: msg-0",
		"被回复消息角色: assistant",
		"被回复发送者ID: bot-1",
		"被回复发送者用户名: helperbot",
		"被回复消息内容:\nearlier answer",
	}
	for _, check := range checks {
		if !strings.Contains(rendered, check) {
			t.Fatalf("expected %q in rendered message, got %q", check, rendered)
		}
	}
}

func TestMessageRecordRenderForModelIncludesVisualReferences(t *testing.T) {
	record := MessageRecord{
		Role:    "user",
		Content: "看看这个",
		Time:    time.Date(2026, 3, 18, 12, 34, 56, 0, time.UTC),
		Author: MessageAuthor{
			UserID:      "user-1",
			Username:    "alice",
			DisplayName: "Alice",
		},
		Images: []ImageReference{
			{
				Kind:    "custom_emoji",
				Name:    "smile",
				EmojiID: "123456789012345678",
				URL:     "https://cdn.discordapp.com/emojis/123456789012345678.png?size=128&quality=lossless",
			},
			{
				Kind:        "attachment",
				Name:        "pic.png",
				URL:         "https://example.com/pic.png",
				ContentType: "image/png",
			},
		},
	}

	rendered := record.RenderForModel()
	checks := []string{
		"附带图片/表情:",
		"自定义表情 <:smile:123456789012345678>",
		"图片附件 pic.png (image/png)",
	}
	for _, check := range checks {
		if !strings.Contains(rendered, check) {
			t.Fatalf("expected %q in rendered message, got %q", check, rendered)
		}
	}
}
