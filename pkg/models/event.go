package models

import (
	"time"

	"github.com/google/uuid"
)

// AudioChunk 오디오 데이터 청크를 나타냅니다
type AudioChunk struct {
	ID         string    `json:"id"`
	UserID     string    `json:"user_id"`
	RoomID     string    `json:"room_id"`
	Timestamp  time.Time `json:"timestamp"`
	Data       []byte    `json:"data"`
	SampleRate int       `json:"sample_rate"`
	Channels   int       `json:"channels"`
}

// NewAudioChunk 새로운 AudioChunk 인스턴스를 생성합니다
func NewAudioChunk(userID, roomID string, data []byte, sampleRate, channels int) *AudioChunk {
	return &AudioChunk{
		ID:         uuid.New().String(),
		UserID:     userID,
		RoomID:     roomID,
		Timestamp:  time.Now(),
		Data:       data,
		SampleRate: sampleRate,
		Channels:   channels,
	}
}

// TranscriptEvent 음성을 텍스트로 변환한 결과를 나타냅니다
type TranscriptEvent struct {
	ID           string    `json:"id"`
	AudioChunkID string    `json:"audio_chunk_id"`
	UserID       string    `json:"user_id"`
	RoomID       string    `json:"room_id"`
	Timestamp    time.Time `json:"timestamp"`
	Text         string    `json:"text"`
	Language     string    `json:"language"`
	Confidence   float64   `json:"confidence"`
}

// NewTranscriptEvent 새로운 TranscriptEvent 인스턴스를 생성합니다
func NewTranscriptEvent(audioChunkID, userID, roomID, text, language string, confidence float64) *TranscriptEvent {
	return &TranscriptEvent{
		ID:           uuid.New().String(),
		AudioChunkID: audioChunkID,
		UserID:       userID,
		RoomID:       roomID,
		Timestamp:    time.Now(),
		Text:         text,
		Language:     language,
		Confidence:   confidence,
	}
}

// TranslationEvent 번역 결과를 나타냅니다
type TranslationEvent struct {
	ID           string    `json:"id"`
	TranscriptID string    `json:"transcript_id"`
	UserID       string    `json:"user_id"`
	RoomID       string    `json:"room_id"`
	Timestamp    time.Time `json:"timestamp"`
	SourceText   string    `json:"source_text"`
	SourceLang   string    `json:"source_lang"`
	TargetText   string    `json:"target_text"`
	TargetLang   string    `json:"target_lang"`
}

// NewTranslationEvent 새로운 TranslationEvent 인스턴스를 생성합니다
func NewTranslationEvent(transcriptID, userID, roomID, sourceText, sourceLang, targetText, targetLang string) *TranslationEvent {
	return &TranslationEvent{
		ID:           uuid.New().String(),
		TranscriptID: transcriptID,
		UserID:       userID,
		RoomID:       roomID,
		Timestamp:    time.Now(),
		SourceText:   sourceText,
		SourceLang:   sourceLang,
		TargetText:   targetText,
		TargetLang:   targetLang,
	}
}
