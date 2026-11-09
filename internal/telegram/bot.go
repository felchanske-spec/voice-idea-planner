package telegram

import (
	"context"
	"fmt"
	"io"
	"log"
	"net/http"
	"strings"
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
		log.Printf("[Bot ERROR] failed to process idea for user %d: %v", userID, err)
		errStr := err.Error()
		if strings.Contains(errStr, "429") || strings.Contains(errStr, "quota") || strings.Contains(errStr, "limit") {
			b.sendText(chatID, "⚠️ Извините, сервис сейчас перегружен из-за большого количества запросов. Пожалуйста, попробуйте повторить попытку позже.")
			return
		}
		if strings.Contains(errStr, "deadline") || strings.Contains(errStr, "timeout") {
			b.sendText(chatID, "⏱️ Превышено время ожидания ответа от сервера. Пожалуйста, попробуйте отправить вашу идею еще раз.")
			return
		}
		b.sendText(chatID, "❌ К сожалению, сервис временно недоступен из-за технического сбоя. Мы уже работаем над его устранением!")
		return
	}

	fullText := idea.MarkdownText
	const maxMessageSize = 3000

	for len(fullText) > 0 {
		if len(fullText) <= maxMessageSize {
			msg := tgbotapi.NewMessage(chatID, fullText)
			msg.ParseMode = tgbotapi.ModeMarkdown
			if _, err := b.bot.Send(msg); err != nil {
				msg.ParseMode = ""
				_, _ = b.bot.Send(msg)
			}
			break
		}

		splitIdx := maxMessageSize
		if lastNL := strings.LastIndex(fullText[:maxMessageSize], "\n"); lastNL > 500 {
			splitIdx = lastNL
		}

		chunk := fullText[:splitIdx]
		fullText = fullText[splitIdx:]

		msg := tgbotapi.NewMessage(chatID, chunk)
		msg.ParseMode = tgbotapi.ModeMarkdown
		if _, err := b.bot.Send(msg); err != nil {
			log.Printf("[Bot WARNING] non-critical markdown parse fallback triggered for chunk")
			msg.ParseMode = ""
			_, _ = b.bot.Send(msg)
		}
	}

	cleanedMermaid := sanitizeMermaidCode(idea.MermaidCode)

	if cleanedMermaid != "" && strings.Contains(cleanedMermaid, "graph") {
		b.sendText(chatID, "🎨 Составляю графическую схему архитектуры...")

		imgBytes, err := b.planner.GenerateDiagramBytes(ctx, cleanedMermaid)
		if err != nil {
			log.Printf("[Bot WARNING] failed to render mermaid diagram: %v", err)
			b.sendText(chatID, "⚠️ Не удалось отрендерить графическую схему архитектуры из-за синтаксической ошибки ИИ. Но подробный текстовый план доступен выше!")
			return
		}

		fileBytes := tgbotapi.FileBytes{
			Name:  "architecture.png",
			Bytes: imgBytes,
		}

		photoMsg := tgbotapi.NewPhoto(chatID, fileBytes)
		photoMsg.Caption = "🗺️ Схема архитектуры вашего MVP-проекта"

		_, err = b.bot.Send(photoMsg)
		if err != nil {
			log.Printf("[Bot ERROR] failed to send photo message: %v", err)
		}
	}
}

func (b *BotGateway) handleVoice(ctx context.Context, chatID int64, userID int64, voice *tgbotapi.Voice) {
	b.sendText(chatID, "🎙️ Голосовое принято! Скачиваю аудио, перевожу в текст и формирую ИТ-план...")

	fileURL, err := b.bot.GetFileDirectURL(voice.FileID)
	if err != nil {
		log.Printf("[Bot ERROR] failed to get file URL from telegram for user %d: %v", userID, err)
		b.sendText(chatID, "❌ Не удалось получить файл из Telegram. Попробуйте еще раз.")
		return
	}

	resp, err := http.Get(fileURL)
	if err != nil {
		log.Printf("[Bot ERROR] failed to download voice file for user %d: %v", userID, err)
		b.sendText(chatID, "❌ Ошибка скачивания аудио-файла. Сервера Telegram временно недоступны.")
		return
	}
	defer resp.Body.Close()

	audioBytes, err := io.ReadAll(resp.Body)
	if err != nil {
		log.Printf("[Bot ERROR] failed to read voice response body for user %d: %v", userID, err)
		b.sendText(chatID, "❌ Ошибка чтения аудио-данных.")
		return
	}

	transcribedText, err := b.planner.TranscribeVoice(ctx, audioBytes)
	if err != nil {
		log.Printf("[Bot ERROR] voice transcription failed for user %d: %v", userID, err)

		errStr := err.Error()

		if strings.Contains(errStr, "429") || strings.Contains(errStr, "quota") || strings.Contains(errStr, "limit") {
			b.sendText(chatID, "⚠️ Извините, сервис распознавания речи сейчас перегружен. Попробуйте отправить вашу идею текстом.")
			return
		}

		if strings.Contains(errStr, "deadline") || strings.Contains(errStr, "timeout") {
			b.sendText(chatID, "⏱️ Превышено время ожидания ответа от сервера. Пожалуйста, попробуйте надиктовать сообщение еще раз.")
			return
		}

		b.sendText(chatID, "❌ Не удалось распознать речь. Пожалуйста, попробуйте надиктовать сообщение в тихом месте или отправьте текст.")
		return
	}

	b.processTextIdea(ctx, chatID, userID, transcribedText, true)
}

func (b *BotGateway) handleHistory(ctx context.Context, chatID int64, userID int64) {
	b.sendText(chatID, "📚 Загружаю историю ваших проектов...")

	ideas, err := b.planner.GetHistory(ctx, userID)
	if err != nil {
		log.Printf("[Bot ERROR] failed to fetch history: %v", err)
		b.sendText(chatID, "❌ Не удалось загрузить историю проектов. Попробуйте позже.")
		return
	}

	if len(ideas) == 0 {
		b.sendText(chatID, "💡 У вас пока нет сохраненных ИТ-планов. Надиктуйте или напишите мне свою первую идею!")
		return
	}

	var response strings.Builder
	response.WriteString("📋 **Ваши сохраненные проекты:**\n\n")

	for i, idea := range ideas {
		dateStr := idea.CreatedAt.Format("02.01.2006")
		response.WriteString(fmt.Sprintf("%d. **%s** (%s)\n", i+1, idea.Title, dateStr))
	}

	response.WriteString("\n*Вы можете сгенерировать новый план в любое время, просто отправив сообщение!*")

	msg := tgbotapi.NewMessage(chatID, response.String())
	msg.ParseMode = tgbotapi.ModeMarkdown
	_, _ = b.bot.Send(msg)
}

func (b *BotGateway) sendText(chatID int64, text string) {
	msg := tgbotapi.NewMessage(chatID, text)
	_, _ = b.bot.Send(msg)
}

func sanitizeMermaidCode(code string) string {
	cleaned := strings.TrimSpace(code)

	if strings.HasPrefix(cleaned, "```") {
		cleaned = strings.TrimPrefix(cleaned, "```mermaid")
		cleaned = strings.TrimPrefix(cleaned, "```")
		cleaned = strings.TrimSuffix(cleaned, "```")
		cleaned = strings.TrimSpace(cleaned)
	}

	lines := strings.Split(cleaned, "\n")
	var validLines []string
	for _, line := range lines {
		trimmedLine := strings.TrimSpace(line)
		if trimmedLine == "" || strings.HasPrefix(trimmedLine, "%%") {
			continue
		}

		trimmedLine = strings.ReplaceAll(trimmedLine, "--->", "-->")

		validLines = append(validLines, trimmedLine)
	}

	return strings.Join(validLines, "\n")
}
