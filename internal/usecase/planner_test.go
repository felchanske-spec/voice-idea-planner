package usecase_test

import (
	"context"
	"errors"
	"testing"

	"github.com/felchanske-spec/voice-idea-planner/internal/domain"
	"github.com/felchanske-spec/voice-idea-planner/internal/usecase"
	"github.com/felchanske-spec/voice-idea-planner/internal/usecase/mocks"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
)

func TestProcessIdea_TableDriven(t *testing.T) {
	type fields struct {
		setupMockRepo func(m *mocks.IdeaRepository)
		setupMockLLM  func(m *mocks.LLMClient)
	}

	tests := []struct {
		name          string
		userID        int64
		rawText       string
		fields        fields
		wantErr       error
		expectedTitle string
		expectedMVP   string
		expectedRisk  string
	}{
		{
			name:    "Happy Path - Success",
			userID:  123,
			rawText: "i want to make a pet project",
			fields: fields{
				setupMockRepo: func(m *mocks.IdeaRepository) {
					m.On("SaveIdea", mock.Anything, mock.Anything).Return(nil)
				},
				setupMockLLM: func(m *mocks.LLMClient) {
					m.On("TitleGen", mock.Anything, "i want to make a pet project").Return("Test project", nil)
					m.On("MermaidGen", mock.Anything, "i want to make a pet project").Return("graph TD", nil)
					m.On("GenerateSection", mock.Anything, "i want to make a pet project", "mvp").Return("Make MVP", nil)
					m.On("GenerateSection", mock.Anything, "i want to make a pet project", "stack").Return("Go, Postgres", nil)
					m.On("GenerateSection", mock.Anything, "i want to make a pet project", "risk").Return("No risks", nil)
				},
			},
			wantErr:       nil,
			expectedTitle: "Test project",
			expectedMVP:   "Make MVP",
			expectedRisk:  "No risks",
		},
		{
			name:    "Graceful Degradation - one section fails",
			userID:  123,
			rawText: "i want to make a pet project",
			fields: fields{
				setupMockRepo: func(m *mocks.IdeaRepository) {
					m.On("SaveIdea", mock.Anything, mock.Anything).Return(nil)
				},
				setupMockLLM: func(m *mocks.LLMClient) {
					m.On("TitleGen", mock.Anything, "i want to make a pet project").Return("Test project", nil)
					m.On("MermaidGen", mock.Anything, "i want to make a pet project").Return("graph TD", nil)
					m.On("GenerateSection", mock.Anything, "i want to make a pet project", "mvp").Return("Make MVP", nil)
					m.On("GenerateSection", mock.Anything, "i want to make a pet project", "stack").Return("Go, Postgres", nil)
					m.On("GenerateSection", mock.Anything, "i want to make a pet project", "risk").Return("", errors.New("gemini timeout"))
				},
			},
			wantErr:       nil,
			expectedTitle: "Test project",
			expectedMVP:   "Make MVP",
			expectedRisk:  "failed to generate this section",
		},
		{
			name:    "Validation Failure - empty text",
			userID:  123,
			rawText: "",
			fields: fields{
				setupMockRepo: func(m *mocks.IdeaRepository) {},
				setupMockLLM:  func(m *mocks.LLMClient) {},
			},
			wantErr:       domain.ErrEmptyVoiceText,
			expectedTitle: "",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			mockRepo := new(mocks.IdeaRepository)
			mockLLM := new(mocks.LLMClient)
			mockViz := new(mocks.Visualizer)

			tt.fields.setupMockRepo(mockRepo)
			tt.fields.setupMockLLM(mockLLM)

			planner := usecase.NewPlannerUseCase(mockRepo, mockLLM, mockViz)

			res, err := planner.ProcessIdea(context.Background(), tt.userID, tt.rawText)

			if tt.wantErr != nil {
				assert.ErrorIs(t, err, tt.wantErr)
				assert.Nil(t, res)
			} else {
				assert.NoError(t, err)
				assert.NotNil(t, res)
				assert.Equal(t, tt.expectedTitle, res.Title)
				assert.Equal(t, tt.expectedMVP, res.Sections.MVP)
				assert.Equal(t, tt.expectedRisk, res.Sections.Risk)
			}

			mockRepo.AssertExpectations(t)
			mockLLM.AssertExpectations(t)
		})
	}
}
