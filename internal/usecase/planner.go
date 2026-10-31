package usecase

import (
	"context"
	"fmt"
	"log/slog"
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

	ctx, cancel := context.WithTimeout(ctx, 30*time.Second)
	defer cancel()

	idea := &domain.Idea{
		UserID:    userID,
		RawText:   rawText,
		CreatedAt: time.Now(),
	}

	title, err := p.llm.TitleGen(ctx, rawText)
	if err != nil {
		slog.Error("failed to generate title", "error", err)
		idea.Title = "New Project"
	} else {
		idea.Title = title
	}

	mermaid, err := p.llm.MermaidGen(ctx, rawText)
	if err != nil {
		slog.Error("failed to generate mermaid code", "error", err)
		idea.MermaidCode = ""
	} else {
		idea.MermaidCode = mermaid
	}

	var wg sync.WaitGroup
	var mu sync.Mutex
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
				slog.Error("failed to generate section", "error", err)
				res = "failed to generate this section"
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

	idea.MarkdownText = fmt.Sprintf(
		"# 💡 Идея: %s\n\n"+
			"## 🎯 MVP Scope\n%s\n\n"+
			"## 🛠️ Технологический стек\n%s\n\n"+
			"## ⚠️ Риски проекта\n%s",
		idea.Title, idea.Sections.MVP, idea.Sections.Stack, idea.Sections.Risk,
	)
	if err := p.repo.SaveIdea(ctx, idea); err != nil {
		slog.Error("failed to save idea to repository", "error", err)
	}
	return idea, nil
}
