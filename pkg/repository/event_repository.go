package repository

import (
	"context"
	"fmt"

	"github.com/dongjune8931/goSori/pkg/models"
	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/mongo"
	"go.mongodb.org/mongo-driver/mongo/options"
)

// mongoEventRepository MongoDB 기반 Event Repository 구현
type mongoEventRepository struct {
	audioChunks  *mongo.Collection
	transcripts  *mongo.Collection
	translations *mongo.Collection
}

// NewMongoEventRepository 새로운 Event Repository를 생성합니다
func NewMongoEventRepository(db *mongo.Database) EventRepository {
	return &mongoEventRepository{
		audioChunks:  db.Collection("audio_chunks"),
		transcripts:  db.Collection("transcripts"),
		translations: db.Collection("translations"),
	}
}

// SaveAudioChunk AudioChunk를 저장합니다 (Context 기반)
func (r *mongoEventRepository) SaveAudioChunk(ctx context.Context, chunk *models.AudioChunk) error {
	if _, err := r.audioChunks.InsertOne(ctx, chunk); err != nil {
		return fmt.Errorf("AudioChunk 저장 실패: %w", err)
	}
	return nil
}

// SaveTranscript TranscriptEvent를 저장합니다
func (r *mongoEventRepository) SaveTranscript(ctx context.Context, transcript *models.TranscriptEvent) error {
	if _, err := r.transcripts.InsertOne(ctx, transcript); err != nil {
		return fmt.Errorf("TranscriptEvent 저장 실패: %w", err)
	}
	return nil
}

// SaveTranslation TranslationEvent를 저장합니다
func (r *mongoEventRepository) SaveTranslation(ctx context.Context, translation *models.TranslationEvent) error {
	if _, err := r.translations.InsertOne(ctx, translation); err != nil {
		return fmt.Errorf("TranslationEvent 저장 실패: %w", err)
	}
	return nil
}

// GetTranscriptsByRoom 룸의 모든 Transcript를 조회합니다 (최신순 정렬)
func (r *mongoEventRepository) GetTranscriptsByRoom(ctx context.Context, roomID string) ([]*models.TranscriptEvent, error) {
	filter := bson.M{"room_id": roomID}
	opts := options.Find().SetSort(bson.D{{Key: "timestamp", Value: -1}}) // 최신순

	cursor, err := r.transcripts.Find(ctx, filter, opts)
	if err != nil {
		return nil, fmt.Errorf("Transcript 조회 실패: %w", err)
	}
	defer cursor.Close(ctx)

	var transcripts []*models.TranscriptEvent
	if err := cursor.All(ctx, &transcripts); err != nil {
		return nil, fmt.Errorf("Transcript 디코딩 실패: %w", err)
	}

	return transcripts, nil
}

// GetTranslationsByRoom 룸의 모든 Translation을 조회합니다 (최신순 정렬)
func (r *mongoEventRepository) GetTranslationsByRoom(ctx context.Context, roomID string) ([]*models.TranslationEvent, error) {
	filter := bson.M{"room_id": roomID}
	opts := options.Find().SetSort(bson.D{{Key: "timestamp", Value: -1}}) // 최신순

	cursor, err := r.translations.Find(ctx, filter, opts)
	if err != nil {
		return nil, fmt.Errorf("Translation 조회 실패: %w", err)
	}
	defer cursor.Close(ctx)

	var translations []*models.TranslationEvent
	if err := cursor.All(ctx, &translations); err != nil {
		return nil, fmt.Errorf("Translation 디코딩 실패: %w", err)
	}

	return translations, nil
}
