package bot

import (
	"context"
	"fmt"
	"log"
	"strings"
	"time"

	"discordbot/internal/memory"

	"github.com/bwmarrin/discordgo"
)

const typingRefreshInterval = 8 * time.Second
const emojiAnalysisTimeout = 3 * time.Minute

type typingSender func(channelID string) error

type Session struct {
	session        *discordgo.Session
	commandGuildID string
}

func NewSession(token, commandGuildID string, handler *Handler) (*Session, error) {
	session, err := discordgo.New("Bot " + token)
	if err != nil {
		return nil, err
	}
	session.Identify.Intents = discordgo.IntentsGuildMessages | discordgo.IntentsDirectMessages | discordgo.IntentsMessageContent
	session.AddHandler(func(s *discordgo.Session, m *discordgo.MessageCreate) {
		if m.Author == nil || m.Author.Bot {
			return
		}

		content, ok := promptContentForMessage(s, m)
		if ok {
			if !handler.AllowsSpeechForMessage(s, m) {
				return
			}
			handleIncomingMessage(s, m, handler, content)
			return
		}

		content, ok = proactivePromptContentForMessage(m)
		if !ok || !handler.ShouldProactiveReply() {
			return
		}
		if !handler.AllowsSpeechForMessage(s, m) {
			return
		}
		handleIncomingMessage(s, m, handler, content)
	})
	session.AddHandler(func(s *discordgo.Session, i *discordgo.InteractionCreate) {
		if i == nil {
			return
		}

		switch i.Type {
		case discordgo.InteractionApplicationCommand:
			commandData := i.ApplicationCommandData()
			if commandData.Name == "persona" {
				response, err := handler.PersonaPanelCommandResponse(interactionUserID(i))
				if err != nil {
					response = simpleEphemeralInteractionResponse("抱歉，我现在无法打开人设面板。")
				}
				_ = s.InteractionRespond(i.Interaction, response)
				return
			}
			if commandData.Name == "emoji" {
				response, err := handler.EmojiPanelCommandResponse(interactionUserID(i), i.GuildID, guildNameFromSession(s, i.GuildID))
				if err != nil {
					response = simpleEphemeralInteractionResponse("抱歉，我现在无法打开表情管理面板。")
				}
				_ = s.InteractionRespond(i.Interaction, response)
				return
			}
			if commandData.Name == "proactive" {
				response, err := handler.ProactivePanelCommandResponse(interactionUserID(i), speechLocationForInteraction(s, i))
				if err != nil {
					response = simpleEphemeralInteractionResponse("抱歉，我现在无法打开主动回复面板。")
				}
				_ = s.InteractionRespond(i.Interaction, response)
				return
			}
			if commandData.Name == "setup" {
				response, err := handler.handleSetupCommand(interactionUserID(i), speechLocationForInteraction(s, i), commandData.Options)
				if err != nil {
					response = "抱歉，我现在无法保存这个允许发言范围设置。"
				}
				_ = respondToInteraction(s, i.Interaction, response, true)
				return
			}

			ctx, cancel := context.WithTimeout(context.Background(), DefaultTimeout())
			defer cancel()

			response, ephemeral, err := handler.HandleSlashCommand(ctx, interactionUserID(i), commandData)
			if err != nil {
				response = "抱歉，我现在无法处理这个命令。"
				ephemeral = true
			}
			if strings.TrimSpace(response) == "" {
				response = "已完成。"
			}
			_ = respondToInteraction(s, i.Interaction, response, ephemeral)
		case discordgo.InteractionMessageComponent:
			componentData := i.MessageComponentData()
			if isPersonaInteractionCustomID(componentData.CustomID) {
				response, err := handler.PersonaComponentResponse(interactionUserID(i), componentData)
				if err != nil {
					response = simpleEphemeralInteractionResponse("抱歉，我现在无法处理这个人设操作。")
				}
				_ = s.InteractionRespond(i.Interaction, response)
				return
			}
			if isEmojiInteractionCustomID(componentData.CustomID) {
				guildName := guildNameFromSession(s, i.GuildID)
				if isEmojiAsyncInteractionCustomID(componentData.CustomID) {
					_ = s.InteractionRespond(i.Interaction, deferredMessageUpdateResponse())
					go func(interaction *discordgo.Interaction, authorID, guildID, guildName string, customID string) {
						ctx, cancel := context.WithTimeout(context.Background(), emojiAnalysisTimeout)
						defer cancel()

						edit, err := handler.EmojiAnalysisEdit(ctx, authorID, guildID, guildName, customID, func(guildID string) ([]*discordgo.Emoji, error) {
							return s.GuildEmojis(guildID)
						})
						if err != nil {
							edit, _ = handler.emojiPanelEdit(authorID, guildID, guildName, "表情分析失败，请稍后再试。")
						}
						_ = editInteractionResponse(s, interaction, edit)
					}(i.Interaction, interactionUserID(i), i.GuildID, guildName, componentData.CustomID)
					return
				}

				response, err := handler.EmojiComponentResponse(interactionUserID(i), i.GuildID, guildName, componentData)
				if err != nil {
					response = simpleEphemeralInteractionResponse("抱歉，我现在无法处理这个表情管理操作。")
				}
				_ = s.InteractionRespond(i.Interaction, response)
				return
			}
			if isProactiveInteractionCustomID(componentData.CustomID) {
				response, err := handler.ProactiveComponentResponse(interactionUserID(i), speechLocationForInteraction(s, i), componentData)
				if err != nil {
					response = simpleEphemeralInteractionResponse("抱歉，我现在无法处理这个主动回复操作。")
				}
				_ = s.InteractionRespond(i.Interaction, response)
				return
			}
		case discordgo.InteractionModalSubmit:
			modalData := i.ModalSubmitData()
			if isPersonaInteractionCustomID(modalData.CustomID) {
				response, err := handler.PersonaModalResponse(interactionUserID(i), modalData)
				if err != nil {
					response = simpleEphemeralInteractionResponse("抱歉，我现在无法保存这个人设。")
				}
				_ = s.InteractionRespond(i.Interaction, response)
				return
			}
			if isProactiveInteractionCustomID(modalData.CustomID) {
				response, err := handler.ProactiveModalResponse(interactionUserID(i), speechLocationForInteraction(s, i), modalData)
				if err != nil {
					response = simpleEphemeralInteractionResponse("抱歉，我现在无法保存这个主动回复设置。")
				}
				_ = s.InteractionRespond(i.Interaction, response)
				return
			}
		}
	})
	return &Session{session: session, commandGuildID: commandGuildID}, nil
}

func promptContentForMessage(s *discordgo.Session, m *discordgo.MessageCreate) (string, bool) {
	if m == nil || m.Message == nil {
		return "", false
	}

	content := strings.TrimSpace(m.Content)
	hasVisualInput := discordMessageHasVisualInput(m.Message)
	if content == "" {
		if !hasVisualInput {
			return "", false
		}
	}

	if m.GuildID == "" {
		return content, true
	}

	if s.State == nil || s.State.User == nil {
		return "", false
	}

	selfID := s.State.User.ID
	triggeredByMention := mentionsUser(m.Mentions, selfID)
	triggeredByReply := repliesToUser(m.Message, selfID)
	if !triggeredByMention && !triggeredByReply {
		return "", false
	}

	if triggeredByMention {
		content = strings.ReplaceAll(content, "<@"+selfID+">", "")
		content = strings.ReplaceAll(content, "<@!"+selfID+">", "")
	}
	content = strings.TrimSpace(content)
	if content == "" && !hasVisualInput {
		return "", false
	}
	return content, true
}

func proactivePromptContentForMessage(m *discordgo.MessageCreate) (string, bool) {
	if m == nil || m.Message == nil {
		return "", false
	}
	if strings.TrimSpace(m.GuildID) == "" {
		return "", false
	}

	content := strings.TrimSpace(m.Content)
	hasVisualInput := discordMessageHasVisualInput(m.Message)
	if content == "" && !hasVisualInput {
		return "", false
	}
	return content, true
}

func mentionsUser(users []*discordgo.User, userID string) bool {
	for _, user := range users {
		if user != nil && user.ID == userID {
			return true
		}
	}
	return false
}

func repliesToUser(message *discordgo.Message, userID string) bool {
	if message == nil || message.ReferencedMessage == nil || message.ReferencedMessage.Author == nil {
		return false
	}
	return message.ReferencedMessage.Author.ID == userID
}

func messageRecordForDiscordMessage(m *discordgo.MessageCreate, content, botUserID string) memory.MessageRecord {
	record := memory.MessageRecord{
		Role:    "user",
		Content: strings.TrimSpace(content),
	}
	if m == nil || m.Message == nil {
		return record
	}

	record.GuildID = strings.TrimSpace(m.GuildID)
	record.Time = m.Timestamp
	record.Author = authorFromDiscord(m.Author, m.Member)
	record.Images = collectVisualReferences(m.Message, content)
	if reply := replyRecordFromDiscord(m.ReferencedMessage, botUserID); reply != nil {
		record.ReplyTo = reply
	}
	return record
}

func botUserIDFromSession(s *discordgo.Session) string {
	if s == nil || s.State == nil || s.State.User == nil {
		return ""
	}
	return strings.TrimSpace(s.State.User.ID)
}

func authorFromDiscord(user *discordgo.User, member *discordgo.Member) memory.MessageAuthor {
	author := memory.MessageAuthor{}
	if user != nil {
		author.UserID = strings.TrimSpace(user.ID)
		author.Username = strings.TrimSpace(user.Username)
		author.GlobalName = strings.TrimSpace(user.GlobalName)
	}
	if member != nil {
		author.Nick = strings.TrimSpace(member.Nick)
	}
	if author.DisplayName == "" {
		switch {
		case author.Nick != "":
			author.DisplayName = author.Nick
		case author.GlobalName != "":
			author.DisplayName = author.GlobalName
		case author.Username != "":
			author.DisplayName = author.Username
		}
	}
	return author
}

func replyRecordFromDiscord(message *discordgo.Message, botUserID string) *memory.ReplyRecord {
	if message == nil {
		return nil
	}

	role := "user"
	if message.Author != nil && strings.TrimSpace(message.Author.ID) != "" && strings.TrimSpace(message.Author.ID) == strings.TrimSpace(botUserID) {
		role = "assistant"
	}

	return &memory.ReplyRecord{
		MessageID: strings.TrimSpace(message.ID),
		Role:      role,
		Content:   strings.TrimSpace(message.Content),
		Time:      message.Timestamp,
		Author:    authorFromDiscord(message.Author, message.Member),
	}
}

func sendMessageReply(s *discordgo.Session, trigger *discordgo.MessageCreate, content string) (*discordgo.Message, error) {
	if s == nil || trigger == nil || trigger.Message == nil {
		return nil, fmt.Errorf("message reply context is missing")
	}

	replySend := buildReplyMessageSend(trigger.Message, content)
	message, err := s.ChannelMessageSendComplex(trigger.ChannelID, replySend)
	if err == nil {
		return message, nil
	}

	log.Printf("reply send failed, retrying without message reference: guild=%s channel=%s trigger=%s err=%v", strings.TrimSpace(trigger.GuildID), strings.TrimSpace(trigger.ChannelID), strings.TrimSpace(trigger.ID), err)
	return s.ChannelMessageSendComplex(trigger.ChannelID, buildPlainMessageSend(content))
}

func handleIncomingMessage(s *discordgo.Session, m *discordgo.MessageCreate, handler *Handler, content string) {
	ctx, cancel := context.WithTimeout(context.Background(), DefaultTimeout())
	defer cancel()

	stopTyping := startTypingLoop(ctx, m.ChannelID, func(channelID string) error {
		return s.ChannelTyping(channelID)
	}, typingRefreshInterval)
	defer stopTyping()

	response, err := handler.HandleMessageRecord(ctx, m.ChannelID, messageRecordForDiscordMessage(m, content, botUserIDFromSession(s)))
	if err != nil {
		if _, sendErr := sendMessageReply(s, m, "抱歉，我现在无法回应。"); sendErr != nil {
			log.Printf("failed to send fallback error reply: guild=%s channel=%s trigger=%s err=%v", strings.TrimSpace(m.GuildID), strings.TrimSpace(m.ChannelID), strings.TrimSpace(m.ID), sendErr)
		}
		return
	}
	if strings.TrimSpace(response) == "" {
		return
	}
	if _, err := sendMessageReply(s, m, response); err != nil {
		log.Printf("failed to send response message: guild=%s channel=%s trigger=%s err=%v", strings.TrimSpace(m.GuildID), strings.TrimSpace(m.ChannelID), strings.TrimSpace(m.ID), err)
	}
}

func buildReplyMessageSend(trigger *discordgo.Message, content string) *discordgo.MessageSend {
	send := buildPlainMessageSend(content)
	if trigger != nil {
		send.Reference = trigger.Reference()
	}
	return send
}

func buildPlainMessageSend(content string) *discordgo.MessageSend {
	content = strings.TrimSpace(content)
	return &discordgo.MessageSend{
		Content: content,
		AllowedMentions: &discordgo.MessageAllowedMentions{
			Parse:       []discordgo.AllowedMentionType{},
			RepliedUser: false,
		},
	}
}

func startTypingLoop(ctx context.Context, channelID string, send typingSender, interval time.Duration) func() {
	channelID = strings.TrimSpace(channelID)
	if channelID == "" || send == nil {
		return func() {}
	}
	if interval <= 0 {
		interval = typingRefreshInterval
	}

	typingCtx, cancel := context.WithCancel(ctx)
	done := make(chan struct{})
	go func() {
		defer close(done)
		if typingCtx.Err() != nil {
			return
		}

		_ = send(channelID)

		ticker := time.NewTicker(interval)
		defer ticker.Stop()
		for {
			select {
			case <-typingCtx.Done():
				return
			case <-ticker.C:
				_ = send(channelID)
			}
		}
	}()

	return func() {
		cancel()
		<-done
	}
}

func (s *Session) Open() error {
	if err := s.session.Open(); err != nil {
		return err
	}
	if err := s.registerCommands(); err != nil {
		_ = s.session.Close()
		return err
	}
	return nil
}

func (s *Session) Close() error {
	return s.session.Close()
}

func (s *Session) CloseWithContext(ctx context.Context) error {
	done := make(chan error, 1)
	go func() {
		done <- s.session.Close()
	}()
	select {
	case <-ctx.Done():
		return ctx.Err()
	case err := <-done:
		return err
	}
}

func (s *Session) registerCommands() error {
	appID, err := s.applicationID()
	if err != nil {
		return err
	}

	scopes := []string{""}
	if strings.TrimSpace(s.commandGuildID) != "" {
		scopes = append(scopes, s.commandGuildID)
	}
	for _, scope := range scopes {
		if _, err := s.session.ApplicationCommandBulkOverwrite(appID, scope, []*discordgo.ApplicationCommand{}); err != nil {
			return err
		}
	}

	_, err = s.session.ApplicationCommandBulkOverwrite(appID, s.commandGuildID, slashCommands())
	return err
}

func (s *Session) applicationID() (string, error) {
	if s.session.State != nil && s.session.State.User != nil && s.session.State.User.ID != "" {
		return s.session.State.User.ID, nil
	}

	user, err := s.session.User("@me")
	if err != nil {
		return "", err
	}
	if user == nil || user.ID == "" {
		return "", fmt.Errorf("failed to resolve bot application ID")
	}
	if s.session.State != nil {
		s.session.State.User = user
	}
	return user.ID, nil
}

func interactionUserID(i *discordgo.InteractionCreate) string {
	if i == nil {
		return ""
	}
	if i.Member != nil && i.Member.User != nil {
		return i.Member.User.ID
	}
	if i.User != nil {
		return i.User.ID
	}
	return ""
}

func respondToInteraction(s *discordgo.Session, interaction *discordgo.Interaction, content string, ephemeral bool) error {
	data := &discordgo.InteractionResponseData{
		Content: content,
	}
	if ephemeral {
		data.Flags = discordgo.MessageFlagsEphemeral
	}
	return s.InteractionRespond(interaction, &discordgo.InteractionResponse{
		Type: discordgo.InteractionResponseChannelMessageWithSource,
		Data: data,
	})
}

func simpleEphemeralInteractionResponse(content string) *discordgo.InteractionResponse {
	return &discordgo.InteractionResponse{
		Type: discordgo.InteractionResponseChannelMessageWithSource,
		Data: &discordgo.InteractionResponseData{
			Content: strings.TrimSpace(content),
			Flags:   discordgo.MessageFlagsEphemeral,
		},
	}
}

func deferredMessageUpdateResponse() *discordgo.InteractionResponse {
	return &discordgo.InteractionResponse{
		Type: discordgo.InteractionResponseDeferredMessageUpdate,
	}
}

func editInteractionResponse(s *discordgo.Session, interaction *discordgo.Interaction, edit *discordgo.WebhookEdit) error {
	if s == nil || interaction == nil || edit == nil {
		return fmt.Errorf("interaction edit context is missing")
	}
	_, err := s.InteractionResponseEdit(interaction, edit)
	return err
}

func guildNameFromSession(s *discordgo.Session, guildID string) string {
	if s == nil || s.State == nil || strings.TrimSpace(guildID) == "" {
		return ""
	}
	guild, err := s.State.Guild(guildID)
	if err != nil || guild == nil {
		return ""
	}
	return strings.TrimSpace(guild.Name)
}
