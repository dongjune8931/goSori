package config

import (
	"fmt"
	"os"
	"path/filepath"

	"github.com/spf13/viper"
)

// Config 애플리케이션 전체 설정을 담는 구조체
type Config struct {
	Server   ServerConfig   `mapstructure:"server"`
	WebRTC   WebRTCConfig   `mapstructure:"webrtc"`
	AI       AIConfig       `mapstructure:"ai"`
	Pipeline PipelineConfig `mapstructure:"pipeline"`
}

// ServerConfig 서버 설정
type ServerConfig struct {
	Port string `mapstructure:"port"`
	Host string `mapstructure:"host"`
}

// WebRTCConfig WebRTC 설정
type WebRTCConfig struct {
	StunServers []string `mapstructure:"stun_servers"`
}

// AIConfig AI 서비스 설정
type AIConfig struct {
	OpenAI OpenAIConfig `mapstructure:"openai"`
}

// OpenAIConfig OpenAI API 설정
type OpenAIConfig struct {
	APIKey           string `mapstructure:"api_key"`
	STTModel         string `mapstructure:"stt_model"`
	TranslationModel string `mapstructure:"translation_model"`
}

// PipelineConfig 파이프라인 워커 및 큐 설정
type PipelineConfig struct {
	STTWorkers          int `mapstructure:"stt_workers"`
	TranslationWorkers  int `mapstructure:"translation_workers"`
	AudioQueueSize      int `mapstructure:"audio_queue_size"`
	TranscriptQueueSize int `mapstructure:"transcript_queue_size"`
	OutputQueueSize     int `mapstructure:"output_queue_size"`
}

// LoadConfig 설정 파일을 로드하고 환경변수를 병합합니다
func LoadConfig() (*Config, error) {
	// 현재 작업 디렉토리 가져오기
	cwd, err := os.Getwd()
	if err != nil {
		return nil, fmt.Errorf("작업 디렉토리를 가져올 수 없습니다: %w", err)
	}

	// 설정 파일 경로 설정
	configPath := filepath.Join(cwd, "configs")
	viper.AddConfigPath(configPath)
	viper.SetConfigName("config")
	viper.SetConfigType("yaml")

	// 환경변수 자동 바인딩 활성화
	viper.AutomaticEnv()

	// 설정 파일 읽기
	if err := viper.ReadInConfig(); err != nil {
		return nil, fmt.Errorf("설정 파일을 읽을 수 없습니다: %w", err)
	}

	// 환경변수로부터 특정 값 읽기 (config.yaml의 ${ENV_VAR} 형식 대체)
	if apiKey := os.Getenv("OPENAI_API_KEY"); apiKey != "" {
		viper.Set("ai.openai.api_key", apiKey)
	}

	// Config 구조체로 언마샬링
	var cfg Config
	if err := viper.Unmarshal(&cfg); err != nil {
		return nil, fmt.Errorf("설정을 구조체로 변환할 수 없습니다: %w", err)
	}

	// 필수 설정 검증
	if err := validateConfig(&cfg); err != nil {
		return nil, fmt.Errorf("설정 검증 실패: %w", err)
	}

	return &cfg, nil
}

// validateConfig 필수 설정 값들을 검증합니다
func validateConfig(cfg *Config) error {
	if cfg.Server.Port == "" {
		return fmt.Errorf("서버 포트가 설정되지 않았습니다")
	}

	if cfg.AI.OpenAI.APIKey == "" {
		return fmt.Errorf("OpenAI API 키가 설정되지 않았습니다. OPENAI_API_KEY 환경변수를 설정해주세요")
	}

	if cfg.Pipeline.STTWorkers <= 0 {
		return fmt.Errorf("STT 워커 수는 1 이상이어야 합니다")
	}

	if cfg.Pipeline.TranslationWorkers <= 0 {
		return fmt.Errorf("Translation 워커 수는 1 이상이어야 합니다")
	}

	return nil
}
