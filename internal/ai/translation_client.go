package ai

import (
	"context"
	"fmt"

	"github.com/dongjune8931/goSori/pkg/config"
	"github.com/sashabaranov/go-openai"
)

// TranslationClient OpenAI Chat API를 사용하는 번역 클라이언트
type TranslationClient struct {
	client *openai.Client
	model  string
}

// NewTranslationClient 새로운 TranslationClient 인스턴스를 생성합니다
func NewTranslationClient(cfg *config.Config) (*TranslationClient, error) {
	if cfg.AI.OpenAI.APIKey == "" {
		return nil, fmt.Errorf("OpenAI API 키가 설정되지 않았습니다")
	}

	if cfg.AI.OpenAI.TranslationModel == "" {
		return nil, fmt.Errorf("Translation 모델이 설정되지 않았습니다")
	}

	client := openai.NewClient(cfg.AI.OpenAI.APIKey)

	return &TranslationClient{
		client: client,
		model:  cfg.AI.OpenAI.TranslationModel,
	}, nil
}

// Translate 텍스트를 소스 언어에서 타겟 언어로 번역합니다
func (t *TranslationClient) Translate(text, sourceLang, targetLang string) (string, error) {
	if text == "" {
		return "", fmt.Errorf("번역할 텍스트가 비어있습니다")
	}

	if sourceLang == "" || targetLang == "" {
		return "", fmt.Errorf("소스 언어 또는 타겟 언어가 지정되지 않았습니다")
	}

	// 번역 프롬프트 생성
	prompt := fmt.Sprintf("Translate the following text from %s to %s: %s", sourceLang, targetLang, text)

	// Chat Completion 요청 생성
	req := openai.ChatCompletionRequest{
		Model: t.model,
		Messages: []openai.ChatCompletionMessage{
			{
				Role:    openai.ChatMessageRoleUser,
				Content: prompt,
			},
		},
	}

	// API 호출
	ctx := context.Background()
	resp, err := t.client.CreateChatCompletion(ctx, req)
	if err != nil {
		return "", fmt.Errorf("Chat API 호출 실패: %w", err)
	}

	// 응답 검증
	if len(resp.Choices) == 0 {
		return "", fmt.Errorf("API 응답에 결과가 없습니다")
	}

	return resp.Choices[0].Message.Content, nil
}
