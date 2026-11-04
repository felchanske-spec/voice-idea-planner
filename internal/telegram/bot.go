package telegram

import (
	"context"
	"fmt"
	"io"
	"log"
	"net/http"
	"time"

	"github.com/felchanske-spec/voice-idea-planner/internal/usecase"

	tgbotapi "github.com/go-telegram-bot-api/telegram-bot-api/v5"
)

type BotGateway struct {
	bot         *tgbotapi.BotAPI
	planner     *usecase.PlannerUseCase
	genLimiter  *UserLimiter
	histLimiter *UserLimiter
}

func NewBotGateway(token string, planner *usecase.PlannerUseCase) (*BotGateway, error) {
	bot, err := tgbotapi.NewBotAPI(token)
	if err != nil {
		return nil, fmt.Errorf("failed to initialize telegram bot: %w", err)
	}

	return &BotGateway{
		bot:         bot,
		planner:     planner,
		genLimiter:  NewUserLimiter(1 * time.Minute),
		histLimiter: NewUserLimiter(10 * time.Second),
	}, nil
}

func (b *BotGateway) Start(ctx context.Context) {
	u := tgbotapi.NewUpdate(0)
	u.Timeout = 60

	updates := b.bot.GetUpdatesChan(u)
	log.Printf("[Bot] Authorized on account %s. Listening for updates...", b.bot.Self.UserName)

	for update := range updates {
		if update.Message == nil {
			continue
		}
		go b.handleMessage(ctx, update.Message)
	}
}

func (b *BotGateway) handleMessage(ctx context.Context, msg *tgbotapi.Message) {
	chatID := msg.Chat.ID
	userID := msg.From.ID

	if msg.IsCommand() && msg.Command() == "history" {
		if allowed, timeLeft := b.histLimiter.Allow(userID); !allowed {
			b.sendText(chatID, fmt.Sprintf("⚠️ Пожалуйста, подождите %d сек. перед повторным запросом истории.", int(timeLeft.Seconds())))
			return
		}
		b.handleHistory(ctx, chatID, userID)
		return
	}

	if msg.Voice != nil {
		if allowed, timeLeft := b.genLimiter.Allow(userID); !allowed {
			b.sendText(chatID, fmt.Sprintf("⚠️ Запросы к ИИ ограничены. Подождите еще %d сек.", int(timeLeft.Seconds())))
			return
		}
		b.handleVoice(ctx, chatID, userID, msg.Voice)
		return
	}

	if msg.Text != "" {
		if allowed, timeLeft := b.genLimiter.Allow(userID); !allowed {
			b.sendText(chatID, fmt.Sprintf("⚠️ Запросы к ИИ ограничены. Подождите еще %d сек.", int(timeLeft.Seconds())))
			return
		}
		b.processTextIdea(ctx, chatID, userID, msg.Text, false)
		return
	}

	b.sendText(chatID, "💡 Пожалуйста, отправьте мне текстовую задумку или надиктуйте её голосовым сообщением!")
}

func (b *BotGateway) processTextIdea(ctx context.Context, chatID int64, userID int64, text string, isVoice bool) {
	if !isVoice {
		b.sendText(chatID, "🚀 Идея принята! Начал анализировать и составлять ИТ-план. Это займет около 5 секунд...")
	}

	idea, err := b.planner.ProcessIdea(ctx, userID, text)
	if err != nil {
		log.Printf("[Bot ERROR] failed to process idea: %v", err)
		b.sendText(chatID, fmt.Sprintf("❌ Ошибка генерации плана: %v. Пожалуйста, попробуйте позже.", err.Error()))
		return
	}

	msg := tgbotapi.NewMessage(chatID, idea.MarkdownText)
	msg.ParseMode = tgbotapi.ModeMarkdown
	_, _ = b.bot.Send(msg)
}

func (b *BotGateway) handleVoice(ctx context.Context, chatID int64, userID int64, voice *tgbotapi.Voice) {
	b.sendText(chatID, "🎙️ Голосовое принято! Скачиваю аудио, перевожу в текст и формирую ИТ-план...")

	fileURL, err := b.bot.GetFileDirectURL(voice.FileID)
	if err != nil {
		log.Printf("[Bot ERROR] failed to get file URL from telegram: %v", err)
		b.sendText(chatID, "❌ Не удалось получить файл из Telegram. Попробуйте еще раз.")
		return
	}

	resp, err := http.Get(fileURL)
	if err != nil {
		log.Printf("[Bot ERROR] failed to download voice file: %v", err)
		b.sendText(chatID, "❌ Ошибка скачивания аудио-файла. Сервера Telegram временно недоступны.")
		return
	}
	defer resp.Body.Close()

	audioBytes, err := io.ReadAll(resp.Body)
	if err != nil {
		log.Printf("[Bot ERROR] failed to read voice response body: %v", err)
		b.sendText(chatID, "❌ Ошибка чтения аудио-данных.")
		return
	}

	transcribedText, err := b.planner.TranscribeVoice(ctx, audioBytes)
	if err != nil {
		log.Printf("[Bot ERROR] voice transcription failed: %v", err)
		b.sendText(chatID, "❌ Не удалось распознать речь. Возможно, запись слишком тихая, шумная или не содержит слов.")
		return
	}

	b.processTextIdea(ctx, chatID, userID, transcribedText, true)
}

func (b *BotGateway) handleHistory(ctx context.Context, chatID int64, userID int64) {
	b.sendText(chatID, "📚 Загружаю историю ваших проектов...")
	b.sendText(chatID, "У вас пока нет сохраненных идей!")
}

func (b *BotGateway) sendText(chatID int64, text string) {
	msg := tgbotapi.NewMessage(chatID, text)
	_, _ = b.bot.Send(msg)
}
