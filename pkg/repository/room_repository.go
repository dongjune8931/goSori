package repository

import (
	"context"
	"fmt"
	"time"

	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/mongo"
)

// mongoRoomRepository MongoDB 기반 Room Repository 구현
// Go의 구조체 임베딩으로 공통 로직 재사용 가능
type mongoRoomRepository struct {
	collection *mongo.Collection
}

// NewMongoRoomRepository 새로운 Room Repository를 생성합니다
func NewMongoRoomRepository(db *mongo.Database) RoomRepository {
	return &mongoRoomRepository{
		collection: db.Collection("rooms"),
	}
}

// Create 새로운 룸을 생성합니다 (Context 기반 타임아웃)
func (r *mongoRoomRepository) Create(ctx context.Context, room *Room) error {
	room.CreatedAt = time.Now().Unix()
	room.UpdatedAt = time.Now().Unix()

	if _, err := r.collection.InsertOne(ctx, room); err != nil {
		return fmt.Errorf("룸 생성 실패: %w", err)
	}

	return nil
}

// FindByID ID로 룸을 조회합니다
func (r *mongoRoomRepository) FindByID(ctx context.Context, id string) (*Room, error) {
	var room Room

	filter := bson.M{"_id": id}
	if err := r.collection.FindOne(ctx, filter).Decode(&room); err != nil {
		if err == mongo.ErrNoDocuments {
			return nil, fmt.Errorf("룸을 찾을 수 없습니다: %s", id)
		}
		return nil, fmt.Errorf("룸 조회 실패: %w", err)
	}

	return &room, nil
}

// Delete 룸을 삭제합니다
func (r *mongoRoomRepository) Delete(ctx context.Context, id string) error {
	filter := bson.M{"_id": id}

	result, err := r.collection.DeleteOne(ctx, filter)
	if err != nil {
		return fmt.Errorf("룸 삭제 실패: %w", err)
	}

	if result.DeletedCount == 0 {
		return fmt.Errorf("룸을 찾을 수 없습니다: %s", id)
	}

	return nil
}

// AddClient 룸에 클라이언트를 추가합니다
func (r *mongoRoomRepository) AddClient(ctx context.Context, roomID, clientID string) error {
	filter := bson.M{"_id": roomID}
	update := bson.M{
		"$addToSet": bson.M{"clients": clientID}, // $addToSet: 중복 방지
		"$set":      bson.M{"updated_at": time.Now().Unix()},
	}

	result, err := r.collection.UpdateOne(ctx, filter, update)
	if err != nil {
		return fmt.Errorf("클라이언트 추가 실패: %w", err)
	}

	if result.MatchedCount == 0 {
		return fmt.Errorf("룸을 찾을 수 없습니다: %s", roomID)
	}

	return nil
}

// RemoveClient 룸에서 클라이언트를 제거합니다
func (r *mongoRoomRepository) RemoveClient(ctx context.Context, roomID, clientID string) error {
	filter := bson.M{"_id": roomID}
	update := bson.M{
		"$pull": bson.M{"clients": clientID}, // $pull: 배열에서 제거
		"$set":  bson.M{"updated_at": time.Now().Unix()},
	}

	result, err := r.collection.UpdateOne(ctx, filter, update)
	if err != nil {
		return fmt.Errorf("클라이언트 제거 실패: %w", err)
	}

	if result.MatchedCount == 0 {
		return fmt.Errorf("룸을 찾을 수 없습니다: %s", roomID)
	}

	return nil
}

// GetClients 룸의 모든 클라이언트를 조회합니다
func (r *mongoRoomRepository) GetClients(ctx context.Context, roomID string) ([]string, error) {
	room, err := r.FindByID(ctx, roomID)
	if err != nil {
		return nil, err
	}

	return room.Clients, nil
}
