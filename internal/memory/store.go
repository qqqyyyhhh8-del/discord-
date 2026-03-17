package memory

import (
	"context"
	"fmt"
	"log"
	"math"
	"sort"
	"strings"
	"sync"
	"time"
)

var utc8Location = time.FixedZone("UTC+8", 8*60*60)

type MessageAuthor struct {
	UserID      string
	Username    string
	GlobalName  string
	Nick        string
	DisplayName string
}

type ReplyRecord struct {
	MessageID string
	Role      string
	Content   string
	Time      time.Time
	Author    MessageAuthor
}

type ImageReference struct {
	Kind        string
	Name        string
	EmojiID     string
	URL         string
	Animated    bool
	ContentType string
}

type MessageRecord struct {
	Role    string
	GuildID string
	Content string
	Time    time.Time
	Author  MessageAuthor
	ReplyTo *ReplyRecord
	Images  []ImageReference
}

type VectorRecord struct {
	Content   string
	Rendered  string
	Embedding []float64
	Time      time.Time
}

type ChannelMemory struct {
	Summary  string
	Messages []MessageRecord
	Vectors  []VectorRecord
}

type EmbedFn func(ctx context.Context, input string) ([]float64, error)

type Store struct {
	mu      sync.Mutex
	byChID  map[string]*ChannelMemory
	embedFn EmbedFn
}

func NewStore(embedFn EmbedFn) *Store {
	return &Store{
		byChID:  make(map[string]*ChannelMemory),
		embedFn: embedFn,
	}
}

func (s *Store) AddMessage(ctx context.Context, chID, role, content string) {
	s.AddRecord(ctx, chID, MessageRecord{
		Role:    role,
		Content: content,
	})
}

func (s *Store) AddRecord(ctx context.Context, chID string, record MessageRecord) {
	record = normalizeMessageRecord(record)
	mem := s.getOrCreate(chID)
	s.mu.Lock()
	mem.Messages = append(mem.Messages, record)
	s.mu.Unlock()
	if record.Role == "user" && record.Content != "" {
		go func() {
			indexCtx, cancel := context.WithTimeout(context.WithoutCancel(ctx), 30*time.Second)
			defer cancel()
			s.indexMessage(indexCtx, chID, record)
		}()
	}
}

func (s *Store) SummaryAndRecent(chID string) (summary string, messages []MessageRecord) {
	s.mu.Lock()
	defer s.mu.Unlock()
	mem := s.byChID[chID]
	if mem == nil {
		return "", nil
	}
	return mem.Summary, append([]MessageRecord(nil), mem.Messages...)
}

func (s *Store) SetSummary(chID, summary string) {
	s.mu.Lock()
	defer s.mu.Unlock()
	mem := s.byChID[chID]
	if mem == nil {
		mem = &ChannelMemory{}
		s.byChID[chID] = mem
	}
	mem.Summary = summary
}

func (s *Store) TrimHistory(chID string, keep int) {
	s.mu.Lock()
	defer s.mu.Unlock()
	mem := s.byChID[chID]
	if mem == nil {
		return
	}
	if len(mem.Messages) > keep {
		mem.Messages = mem.Messages[len(mem.Messages)-keep:]
	}
}

func (s *Store) TopK(chID string, query []float64, k int) []string {
	records := s.TopKRecords(chID, query, k)
	results := make([]string, 0, len(records))
	for _, record := range records {
		results = append(results, record.Content)
	}
	return results
}

func (s *Store) TopKRecords(chID string, query []float64, k int) []VectorRecord {
	s.mu.Lock()
	mem := s.byChID[chID]
	if mem == nil {
		s.mu.Unlock()
		return nil
	}
	vectors := append([]VectorRecord(nil), mem.Vectors...)
	s.mu.Unlock()
	type scored struct {
		vec   VectorRecord
		score float64
	}
	scoredList := make([]scored, 0, len(vectors))
	for _, vec := range vectors {
		score := cosineSimilarity(query, vec.Embedding)
		scoredList = append(scoredList, scored{vec: vec, score: score})
	}
	sort.Slice(scoredList, func(i, j int) bool {
		return scoredList[i].score > scoredList[j].score
	})
	if len(scoredList) > k {
		scoredList = scoredList[:k]
	}
	results := make([]VectorRecord, 0, len(scoredList))
	for _, item := range scoredList {
		results = append(results, item.vec)
	}
	return results
}

func (s *Store) getOrCreate(chID string) *ChannelMemory {
	s.mu.Lock()
	defer s.mu.Unlock()
	mem := s.byChID[chID]
	if mem == nil {
		mem = &ChannelMemory{}
		s.byChID[chID] = mem
	}
	return mem
}

func (s *Store) indexMessage(ctx context.Context, chID string, record MessageRecord) {
	embedding, err := s.embedFn(ctx, record.Content)
	if err != nil {
		log.Printf("embedding error: %v", err)
		return
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	mem := s.byChID[chID]
	if mem == nil {
		mem = &ChannelMemory{}
		s.byChID[chID] = mem
	}
	mem.Vectors = append(mem.Vectors, VectorRecord{
		Content:   record.Content,
		Rendered:  record.RenderForModel(),
		Embedding: embedding,
		Time:      record.Time,
	})
	if len(mem.Vectors) > 200 {
		mem.Vectors = mem.Vectors[len(mem.Vectors)-200:]
	}
}

func (r MessageRecord) RenderForModel() string {
	record := normalizeMessageRecord(r)
	lines := []string{
		fmt.Sprintf("时间(UTC+8): %s", record.Time.In(utc8Location).Format("2006-01-02 15:04:05")),
	}

	switch record.Role {
	case "user":
		lines = append(lines,
			"发送者ID: "+valueOrUnknown(record.Author.UserID),
			"发送者用户名: "+valueOrUnknown(record.Author.Username),
			"发送者全局名: "+valueOrUnknown(record.Author.GlobalName),
			"发送者频道昵称: "+valueOrUnknown(record.Author.Nick),
			"发送者显示名: "+valueOrUnknown(record.Author.DisplayName),
		)
	case "assistant":
		lines = append(lines, "发送者: 机器人")
	default:
		lines = append(lines, "发送者角色: "+valueOrUnknown(record.Role))
	}

	lines = append(lines, "内容:", record.Content)
	if len(record.Images) > 0 {
		lines = append(lines, "", "附带图片/表情:")
		for index, image := range record.Images {
			lines = append(lines, fmt.Sprintf("%d. %s", index+1, renderImageReference(image)))
		}
	}
	if record.ReplyTo != nil {
		reply := normalizeReplyRecord(*record.ReplyTo)
		lines = append(lines,
			"",
			"这条消息是在回复以下消息:",
			"被回复消息ID: "+valueOrUnknown(reply.MessageID),
			fmt.Sprintf("被回复消息时间(UTC+8): %s", reply.Time.In(utc8Location).Format("2006-01-02 15:04:05")),
			"被回复消息角色: "+valueOrUnknown(reply.Role),
			"被回复发送者ID: "+valueOrUnknown(reply.Author.UserID),
			"被回复发送者用户名: "+valueOrUnknown(reply.Author.Username),
			"被回复发送者全局名: "+valueOrUnknown(reply.Author.GlobalName),
			"被回复发送者频道昵称: "+valueOrUnknown(reply.Author.Nick),
			"被回复发送者显示名: "+valueOrUnknown(reply.Author.DisplayName),
			"被回复消息内容:",
			reply.Content,
		)
	}
	return strings.Join(lines, "\n")
}

func normalizeMessageRecord(record MessageRecord) MessageRecord {
	record.Role = strings.TrimSpace(record.Role)
	record.GuildID = strings.TrimSpace(record.GuildID)
	record.Content = strings.TrimSpace(record.Content)
	if record.Time.IsZero() {
		record.Time = time.Now()
	}
	record.Author = normalizeAuthor(record.Author)
	if record.ReplyTo != nil {
		reply := normalizeReplyRecord(*record.ReplyTo)
		record.ReplyTo = &reply
	}
	record.Images = normalizeImageReferences(record.Images)
	return record
}

func normalizeReplyRecord(reply ReplyRecord) ReplyRecord {
	reply.MessageID = strings.TrimSpace(reply.MessageID)
	reply.Role = strings.TrimSpace(reply.Role)
	reply.Content = strings.TrimSpace(reply.Content)
	if reply.Role == "" {
		reply.Role = "unknown"
	}
	if reply.Time.IsZero() {
		reply.Time = time.Now()
	}
	reply.Author = normalizeAuthor(reply.Author)
	return reply
}

func normalizeAuthor(author MessageAuthor) MessageAuthor {
	author.UserID = strings.TrimSpace(author.UserID)
	author.Username = strings.TrimSpace(author.Username)
	author.GlobalName = strings.TrimSpace(author.GlobalName)
	author.Nick = strings.TrimSpace(author.Nick)
	author.DisplayName = strings.TrimSpace(author.DisplayName)
	if author.DisplayName == "" {
		switch {
		case author.Nick != "":
			author.DisplayName = author.Nick
		case author.GlobalName != "":
			author.DisplayName = author.GlobalName
		case author.Username != "":
			author.DisplayName = author.Username
		case author.UserID != "":
			author.DisplayName = author.UserID
		}
	}
	return author
}

func valueOrUnknown(value string) string {
	value = strings.TrimSpace(value)
	if value == "" {
		return "未设置"
	}
	return value
}

func normalizeImageReferences(images []ImageReference) []ImageReference {
	if len(images) == 0 {
		return nil
	}

	normalized := make([]ImageReference, 0, len(images))
	seen := make(map[string]struct{}, len(images))
	for _, image := range images {
		image.Kind = strings.TrimSpace(image.Kind)
		image.Name = strings.TrimSpace(image.Name)
		image.EmojiID = strings.TrimSpace(image.EmojiID)
		image.URL = strings.TrimSpace(image.URL)
		image.ContentType = strings.TrimSpace(image.ContentType)
		if image.URL == "" {
			continue
		}
		key := image.Kind + "\x00" + image.EmojiID + "\x00" + image.URL
		if _, ok := seen[key]; ok {
			continue
		}
		seen[key] = struct{}{}
		normalized = append(normalized, image)
	}
	return normalized
}

func renderImageReference(image ImageReference) string {
	url := valueOrUnknown(image.URL)
	switch image.Kind {
	case "custom_emoji":
		tag := valueOrUnknown(image.Name)
		if strings.TrimSpace(image.EmojiID) != "" && strings.TrimSpace(image.Name) != "" {
			if image.Animated {
				tag = fmt.Sprintf("<a:%s:%s>", image.Name, image.EmojiID)
			} else {
				tag = fmt.Sprintf("<:%s:%s>", image.Name, image.EmojiID)
			}
		}
		return fmt.Sprintf("自定义表情 %s, 图片URL: %s", tag, url)
	case "attachment":
		label := valueOrUnknown(image.Name)
		if strings.TrimSpace(image.ContentType) != "" {
			label += " (" + image.ContentType + ")"
		}
		return fmt.Sprintf("图片附件 %s, 图片URL: %s", label, url)
	default:
		return fmt.Sprintf("图片资源 %s, 图片URL: %s", valueOrUnknown(image.Name), url)
	}
}

func cosineSimilarity(a, b []float64) float64 {
	if len(a) == 0 || len(b) == 0 || len(a) != len(b) {
		return 0
	}
	var dot, normA, normB float64
	for i := range a {
		dot += a[i] * b[i]
		normA += a[i] * a[i]
		normB += b[i] * b[i]
	}
	if normA == 0 || normB == 0 {
		return 0
	}
	return dot / (math.Sqrt(normA) * math.Sqrt(normB))
}
