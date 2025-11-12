package repository

import (
	"context"

	"github.com/dongjune8931/goSori/pkg/models"
)

// Go Interface를 활용한 Repository 패턴
// - 추상화: 구현 세부사항 숨김
// - 테스트 용이성: Mock 구현 가능
// - 유연성: MongoDB 외 다른 DB로 교체 가능

// RoomRepository 룸 관련 데이터 액세스 인터페이스
type RoomRepository interface {
	// Create 새로운 룸을 생성합니다 (Context 기반 타임아웃)
	Create(ctx context.Context, room *Room) error

	// FindByID ID로 룸을 조회합니다
	FindByID(ctx context.Context, id string) (*Room, error)

	// Delete 룸을 삭제합니다
	Delete(ctx context.Context, id string) error

	// AddClient 룸에 클라이언트를 추가합니다
	AddClient(ctx context.Context, roomID, clientID string) error

	// RemoveClient 룸에서 클라이언트를 제거합니다
	RemoveClient(ctx context.Context, roomID, clientID string) error

	// GetClients 룸의 모든 클라이언트를 조회합니다
	GetClients(ctx context.Context, roomID string) ([]string, error)
}

// EventRepository 이벤트 관련 데이터 액세스 인터페이스
type EventRepository interface {
	// SaveAudioChunk AudioChunk를 저장합니다
	SaveAudioChunk(ctx context.Context, chunk *models.AudioChunk) error

	// SaveTranscript TranscriptEvent를 저장합니다
	SaveTranscript(ctx context.Context, transcript *models.TranscriptEvent) error

	// SaveTranslation TranslationEvent를 저장합니다
	SaveTranslation(ctx context.Context, translation *models.TranslationEvent) error

	// GetTranscriptsByRoom 룸의 모든 Transcript를 조회합니다
	GetTranscriptsByRoom(ctx context.Context, roomID string) ([]*models.TranscriptEvent, error)

	// GetTranslationsByRoom 룸의 모든 Translation을 조회합니다
	GetTranslationsByRoom(ctx context.Context, roomID string) ([]*models.TranslationEvent, error)
}

// Room MongoDB에 저장될 룸 문서
type Room struct {
	ID        string   `bson:"_id"`
	CreatedAt int64    `bson:"created_at"`
	UpdatedAt int64    `bson:"updated_at"`
	Clients   []string `bson:"clients"`
}
