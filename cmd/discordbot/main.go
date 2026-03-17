package main

import (
	"context"
	"log"
	"os"
	"os/signal"
	"syscall"
	"time"

	"discordbot/internal/bot"
	"discordbot/internal/config"
	"discordbot/internal/memory"
	"discordbot/internal/openai"
	"discordbot/internal/runtimecfg"
)

func main() {
	cfg, err := config.Load()
	if err != nil {
		log.Fatal(err)
	}

	openAI := openai.NewClient(cfg.OpenAI)
	store := memory.NewStore(openAI.Embed)
	runtimeStore, err := runtimecfg.Open(cfg.Bot.ConfigFilePath)
	if err != nil {
		log.Fatal(err)
	}
	var rerankFn bot.RerankFn
	if openAI.CanRerank() {
		rerankFn = openAI.Rerank
	}

	handler := bot.NewHandler(cfg.Bot, openAI.Chat, openAI.Embed, rerankFn, store, runtimeStore)

	session, err := bot.NewSession(cfg.Bot.DiscordToken, cfg.Bot.CommandGuildID, handler)
	if err != nil {
		log.Fatal(err)
	}
	defer session.Close()

	if err := session.Open(); err != nil {
		log.Fatal(err)
	}
	log.Println("Bot is running. Press CTRL-C to exit.")

	stop := make(chan os.Signal, 1)
	signal.Notify(stop, syscall.SIGINT, syscall.SIGTERM)
	<-stop

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	_ = session.CloseWithContext(ctx)
}
