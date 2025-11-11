package ai

import (
	"context"
	"fmt"
	"os"

	"github.com/dongjune8931/goSori/pkg/config"
	openai "github.com/sashabaranov/go-openai"
)

// STTClient OpenAI Whisper API를 사용하는 음성-텍스트 변환 클라이언트
type STTClient struct {
	client *openai.Client
	model  string
}

// NewSTTClient 새로운 STTClient 인스턴스를 생성합니다
func NewSTTClient(cfg *config.Config) (*STTClient, error) {
	if cfg.AI.OpenAI.APIKey == "" {
		return nil, fmt.Errorf("OpenAI API 키가 설정되지 않았습니다")
	}

	if cfg.AI.OpenAI.STTModel == "" {
		return nil, fmt.Errorf("STT 모델이 설정되지 않았습니다")
	}

	client := openai.NewClient(cfg.AI.OpenAI.APIKey)

	return &STTClient{
		client: client,
		model:  cfg.AI.OpenAI.STTModel,
	}, nil
}

// Transcribe 오디오 데이터를 텍스트로 변환합니다
func (s *STTClient) Transcribe(audioData []byte) (string, error) {
	if len(audioData) == 0 {
		return "", fmt.Errorf("오디오 데이터가 비어있습니다")
	}

	// 임시 파일 생성
	tempFile, err := s.createTempAudioFile(audioData)
	if err != nil {
		return "", fmt.Errorf("임시 오디오 파일 생성 실패: %w", err)
	}
	defer os.Remove(tempFile) // 함수 종료 시 임시 파일 삭제

	// Whisper API 요청 생성
	req := openai.AudioRequest{
		Model:    s.model,
		FilePath: tempFile,
	}

	// API 호출
	ctx := context.Background()
	resp, err := s.client.CreateTranscription(ctx, req)
	if err != nil {
		return "", fmt.Errorf("Whisper API 호출 실패: %w", err)
	}

	return resp.Text, nil
}

// createTempAudioFile 오디오 데이터를 임시 파일로 저장합니다
func (s *STTClient) createTempAudioFile(audioData []byte) (string, error) {
	// 임시 디렉토리에 파일 생성
	tempDir := os.TempDir()

	// 임시 파일 생성
	file, err := os.CreateTemp(tempDir, "audio_*.wav")
	if err != nil {
		return "", fmt.Errorf("임시 파일 생성 실패: %w", err)
	}
	defer file.Close()

	// 오디오 데이터 쓰기
	if _, err := file.Write(audioData); err != nil {
		os.Remove(file.Name())
		return "", fmt.Errorf("임시 파일 쓰기 실패: %w", err)
	}

	return file.Name(), nil
}
