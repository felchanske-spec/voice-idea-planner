package usecase

import (
	"context"
	"fmt"
	"sync"
	"time"

	"github.com/felchanske-spec/voice-idea-planner/internal/domain"
)

type IdeaRepository interface {
	SaveIdea(ctx context.Context, idea *domain.Idea) error
	GetIdeasByUser(ctx context.Context, userID int64) ([]domain.Idea, error)
}

type LLMClient interface {
	GenerateSection(ctx context.Context, rawText string, sectionType string) (string, error)
	TitleGen(ctx context.Context, rawText string) (string, error)
	MermaidGen(ctx context.Context, rawText string) (string, error)
	Transcription(ctx context.Context, audioBytes []byte) (string, error)
}

type Visualizer interface {
	GenerateDiagram(ctx context.Context, mermaidCode string) ([]byte, error)
}

type PlannerUseCase struct {
	repo       IdeaRepository
	llm        LLMClient
	visualizer Visualizer
}

func NewPlannerUseCase(repo IdeaRepository, llm LLMClient, visualizer Visualizer) *PlannerUseCase {
	return &PlannerUseCase{
		repo:       repo,
		llm:        llm,
		visualizer: visualizer,
	}
}

func (p *PlannerUseCase) ProcessIdea(ctx context.Context, userID int64, rawText string) (*domain.Idea, error) {
	if rawText == "" {
		return nil, domain.ErrEmptyVoiceText
	}

	ctx, cancel := context.WithTimeout(ctx, 60*time.Second)
	defer cancel()

	idea := &domain.Idea{
		UserID:    userID,
		RawText:   rawText,
		CreatedAt: time.Now(),
	}

	title, err := p.llm.TitleGen(ctx, rawText)
	if err != nil {
		return nil, fmt.Errorf("failed to generate title: %w", err)
	}
	idea.Title = title

	mermaid, err := p.llm.MermaidGen(ctx, rawText)
	if err != nil {
		return nil, fmt.Errorf("failed to generate mermaid architecture: %w", err)
	}
	idea.MermaidCode = mermaid

	var wg sync.WaitGroup
	var mu sync.Mutex
	var genError error

	sem := make(chan struct{}, 2)
	sectionsToGen := []string{"mvp", "stack", "risk"}

	for _, secType := range sectionsToGen {
		wg.Add(1)

		go func(sType string) {
			defer wg.Done()

			sem <- struct{}{}
			defer func() { <-sem }()

			res, err := p.llm.GenerateSection(ctx, rawText, sType)
			if err != nil {
				mu.Lock()
				genError = fmt.Errorf("failed to generate section %s: %w", sType, err)
				mu.Unlock()
				return
			}

			mu.Lock()
			switch sType {
			case "mvp":
				idea.Sections.MVP = res
			case "stack":
				idea.Sections.Stack = res
			case "risk":
				idea.Sections.Risk = res
			}
			mu.Unlock()
		}(secType)
	}
	wg.Wait()

	if genError != nil {
		return nil, genError
	}

	idea.MarkdownText = fmt.Sprintf(
		"# 💡 Идея проекта: %s\n\n"+
			"## 🎯 MVP Scope\n%s\n\n"+
			"## 🛠️ Технологический стек\n%s\n\n"+
			"## ⚠️ Риски проекта\n%s",
		idea.Title, idea.Sections.MVP, idea.Sections.Stack, idea.Sections.Risk,
	)

	if err := p.repo.SaveIdea(ctx, idea); err != nil {
		return nil, fmt.Errorf("failed to save generated plan to database: %w", err)
	}

	return idea, nil
}

func (p *PlannerUseCase) TranscribeVoice(ctx context.Context, audioBytes []byte) (string, error) {
	return p.llm.Transcription(ctx, audioBytes)
}
