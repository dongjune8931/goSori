package database

import (
	"context"
	"fmt"
	"log"
	"sync"
	"time"

	"github.com/dongjune8931/goSori/pkg/config"
	"go.mongodb.org/mongo-driver/mongo"
	"go.mongodb.org/mongo-driver/mongo/options"
	"go.mongodb.org/mongo-driver/mongo/readpref"
)

var (
	// Go의 sync.Once를 활용한 Singleton 패턴
	clientInstance *mongo.Client
	clientOnce     sync.Once
	clientErr      error
)

// MongoDB 데이터베이스 인스턴스를 담는 구조체
type MongoDB struct {
	Client   *mongo.Client
	Database *mongo.Database
	ctx      context.Context
	cancel   context.CancelFunc
}

// NewMongoDB 새로운 MongoDB 연결을 생성합니다 (Context 기반)
func NewMongoDB(cfg *config.Config) (*MongoDB, error) {
	// Timeout 파싱
	timeout, err := time.ParseDuration(cfg.MongoDB.Timeout)
	if err != nil {
		timeout = 10 * time.Second // 기본값
	}

	// Context with timeout
	ctx, cancel := context.WithTimeout(context.Background(), timeout)

	// sync.Once를 사용한 Singleton: 한 번만 연결
	clientOnce.Do(func() {
		clientOptions := options.Client().
			ApplyURI(cfg.MongoDB.URI).
			SetMaxPoolSize(uint64(cfg.MongoDB.MaxPoolSize))

		// MongoDB 연결
		client, err := mongo.Connect(ctx, clientOptions)
		if err != nil {
			clientErr = fmt.Errorf("MongoDB 연결 실패: %w", err)
			return
		}

		// Ping으로 연결 확인
		if err := client.Ping(ctx, readpref.Primary()); err != nil {
			clientErr = fmt.Errorf("MongoDB Ping 실패: %w", err)
			return
		}

		clientInstance = client
		log.Println("✓ MongoDB 연결 성공")
	})

	if clientErr != nil {
		cancel()
		return nil, clientErr
	}

	// Database 선택
	database := clientInstance.Database(cfg.MongoDB.Database)

	return &MongoDB{
		Client:   clientInstance,
		Database: database,
		ctx:      ctx,
		cancel:   cancel,
	}, nil
}

// Close MongoDB 연결을 안전하게 종료합니다 (Context 기반)
func (m *MongoDB) Close() error {
	defer m.cancel()

	if m.Client != nil {
		// Context with timeout for disconnection
		ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()

		if err := m.Client.Disconnect(ctx); err != nil {
			return fmt.Errorf("MongoDB 연결 종료 실패: %w", err)
		}
		log.Println("✓ MongoDB 연결 종료 완료")
	}

	return nil
}

// GetCollection 컬렉션을 반환합니다
func (m *MongoDB) GetCollection(name string) *mongo.Collection {
	return m.Database.Collection(name)
}

// Ping MongoDB 연결 상태를 확인합니다 (Context 사용)
func (m *MongoDB) Ping(ctx context.Context) error {
	return m.Client.Ping(ctx, readpref.Primary())
}
