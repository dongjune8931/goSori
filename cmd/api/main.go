package main

import (
	"context"
	"fmt"
	"log"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/dongjune8931/goSori/internal/ai"
	"github.com/dongjune8931/goSori/internal/handlers"
	"github.com/dongjune8931/goSori/internal/pipeline"
	"github.com/dongjune8931/goSori/internal/webrtc"
	wsManager "github.com/dongjune8931/goSori/internal/websocket"
	"github.com/dongjune8931/goSori/pkg/config"
	"github.com/dongjune8931/goSori/pkg/database"
	"github.com/dongjune8931/goSori/pkg/models"
	"github.com/dongjune8931/goSori/pkg/repository"
	"github.com/gin-gonic/gin"
	"github.com/gorilla/websocket"
)

var (
	upgrader = websocket.Upgrader{
		CheckOrigin: func(r *http.Request) bool {
			return true // 개발용, 프로덕션에서는 제한 필요
		},
	}
)

// Client 클라이언트 정보
type Client struct {
	ID     string
	RoomID string
	Conn   *websocket.Conn
}

func main() {
	// 1. Config 로드
	log.Println("=== goSori 서버 시작 ===")
	cfg, err := config.LoadConfig()
	if err != nil {
		log.Fatalf("Config 로드 실패: %v", err)
	}
	log.Println("✓ Config 로드 완료")

	// 2. MongoDB 연결 (Context 기반, Singleton)
	mongoDB, err := database.NewMongoDB(cfg)
	if err != nil {
		log.Fatalf("MongoDB 연결 실패: %v", err)
	}
	defer mongoDB.Close()
	log.Println("✓ MongoDB 연결 완료")

	// 3. Repository 생성 (Interface 기반)
	roomRepo := repository.NewMongoRoomRepository(mongoDB.Database)
	eventRepo := repository.NewMongoEventRepository(mongoDB.Database)
	log.Println("✓ Repository 생성 완료")

	// 4. EventSaver 생성 및 시작 (Channel 기반 비동기 저장)
	eventSaver := pipeline.NewEventSaver(eventRepo, 100)
	eventSaver.Start()
	defer eventSaver.Stop()
	log.Println("✓ EventSaver 시작 완료")

	// 5. AI 클라이언트 생성
	sttClient, err := ai.NewSTTClient(cfg)
	if err != nil {
		log.Fatalf("STT 클라이언트 생성 실패: %v", err)
	}
	log.Println("✓ STT 클라이언트 생성 완료")

	translationClient, err := ai.NewTranslationClient(cfg)
	if err != nil {
		log.Fatalf("Translation 클라이언트 생성 실패: %v", err)
	}
	log.Println("✓ Translation 클라이언트 생성 완료")

	// 6. 워커 풀 생성
	sttPool := pipeline.NewSTTWorkerPool(
		cfg.Pipeline.STTWorkers,
		cfg.Pipeline.AudioQueueSize,
		cfg.Pipeline.TranscriptQueueSize,
		sttClient,
	)
	log.Printf("✓ STT Worker Pool 생성 완료 (%d workers)", cfg.Pipeline.STTWorkers)

	translationPool := pipeline.NewTranslationWorkerPool(
		cfg.Pipeline.TranslationWorkers,
		cfg.Pipeline.TranscriptQueueSize,
		cfg.Pipeline.OutputQueueSize,
		translationClient,
	)
	log.Printf("✓ Translation Worker Pool 생성 완료 (%d workers)", cfg.Pipeline.TranslationWorkers)

	// 7. WebSocket ConnectionManager 생성
	connManager := wsManager.NewConnectionManager()
	log.Println("✓ WebSocket Connection Manager 생성 완료")

	// 8. AudioUploadHandler 생성 (AI 테스트용)
	uploadHandler := handlers.NewAudioUploadHandler(nil) // 나중에 pipeline 설정
	log.Println("✓ Audio Upload Handler 생성 완료")

	// 9. AudioPipeline 생성 및 시작 (Channel 기반 파이프라인)
	audioPipeline := pipeline.NewAudioPipeline(
		sttPool,
		translationPool,
		func(event *models.TranslationEvent) {
			// Output Handler: 번역 결과 처리
			log.Printf("📝 번역 완료: [%s→%s] %s → %s",
				event.SourceLang, event.TargetLang,
				event.SourceText, event.TargetText)

			// MongoDB에 비동기 저장 (Go Channel 활용)
			eventSaver.SaveTranslation(event)

			// UploadHandler에 결과 저장 (테스트용)
			uploadHandler.SaveResult(event)

			// WebSocket으로 실시간 브로드캐스트 ✅
			connManager.BroadcastTranslation(event)
		},
	)
	audioPipeline.Start()
	defer audioPipeline.Stop()
	log.Println("✓ Audio Pipeline 시작 완료")

	// 9. AudioHandler 생성
	audioHandler := webrtc.NewAudioHandler(audioPipeline)
	log.Println("✓ Audio Handler 생성 완료")

	// 10. Gin HTTP 서버 설정
	gin.SetMode(gin.ReleaseMode)
	router := gin.Default()

	// UploadHandler에 pipeline 설정
	uploadHandler = handlers.NewAudioUploadHandler(audioPipeline)

	// Health Check
	router.GET("/health", func(c *gin.Context) {
		// Context 기반 MongoDB Ping
		ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
		defer cancel()

		mongoStatus := "healthy"
		if err := mongoDB.Ping(ctx); err != nil {
			mongoStatus = "unhealthy"
		}

		c.JSON(http.StatusOK, gin.H{
			"status":  "healthy",
			"time":    time.Now(),
			"mongodb": mongoStatus,
		})
	})

	// 룸 생성 (Repository 사용)
	router.POST("/room", func(c *gin.Context) {
		roomID := fmt.Sprintf("room-%d", time.Now().Unix())

		// Context with timeout
		ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()

		// Repository를 통해 MongoDB에 저장
		room := &repository.Room{
			ID:      roomID,
			Clients: []string{},
		}
		if err := roomRepo.Create(ctx, room); err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
			return
		}

		log.Printf("✓ 룸 생성: %s", roomID)

		c.JSON(http.StatusCreated, gin.H{
			"room_id": roomID,
		})
	})

	// 룸 조회
	router.GET("/room/:roomId", func(c *gin.Context) {
		roomID := c.Param("roomId")

		ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()

		room, err := roomRepo.FindByID(ctx, roomID)
		if err != nil {
			c.JSON(http.StatusNotFound, gin.H{"error": err.Error()})
			return
		}

		c.JSON(http.StatusOK, room)
	})

	// 🧪 테스트 엔드포인트: 오디오 파일 업로드 및 AI 처리
	router.POST("/test/audio", uploadHandler.UploadAudio)
	router.GET("/test/result/:id", uploadHandler.GetResult)

	// WebSocket Signaling
	router.GET("/ws/:roomId", func(c *gin.Context) {
		roomID := c.Param("roomId")
		clientID := c.Query("clientId")

		if clientID == "" {
			c.JSON(http.StatusBadRequest, gin.H{"error": "clientId required"})
			return
		}

		// WebSocket 업그레이드
		conn, err := upgrader.Upgrade(c.Writer, c.Request, nil)
		if err != nil {
			log.Printf("WebSocket 업그레이드 실패: %v", err)
			return
		}

		// 클라이언트 등록 (Repository 사용)
		ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		if err := roomRepo.AddClient(ctx, roomID, clientID); err != nil {
			log.Printf("클라이언트 추가 실패: %v", err)
			cancel()
			conn.Close()
			return
		}
		cancel()

		client := &Client{
			ID:     clientID,
			RoomID: roomID,
			Conn:   conn,
		}

		// ConnectionManager에 연결 추가
		connManager.AddConnection(roomID, clientID, conn)

		log.Printf("✓ 클라이언트 연결: Room=%s, Client=%s", roomID, clientID)

		// WebSocket 메시지 처리 (Goroutine)
		go handleWebSocket(client, audioHandler, roomRepo, connManager)
	})

	// 10. HTTP 서버 시작
	serverAddr := fmt.Sprintf("%s:%s", cfg.Server.Host, cfg.Server.Port)
	srv := &http.Server{
		Addr:    serverAddr,
		Handler: router,
	}

	go func() {
		log.Printf("🚀 HTTP 서버 시작: %s", serverAddr)
		if err := srv.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			log.Fatalf("HTTP 서버 오류: %v", err)
		}
	}()

	// 11. Graceful Shutdown (Context 기반)
	quit := make(chan os.Signal, 1)
	signal.Notify(quit, syscall.SIGINT, syscall.SIGTERM)
	<-quit

	log.Println("=== 서버 종료 시작 ===")

	// HTTP 서버 종료
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	if err := srv.Shutdown(ctx); err != nil {
		log.Printf("HTTP 서버 종료 오류: %v", err)
	}
	log.Println("✓ HTTP 서버 종료 완료")

	// AudioPipeline 종료
	audioPipeline.Stop()
	log.Println("✓ Audio Pipeline 종료 완료")

	// EventSaver 종료
	eventSaver.Stop()
	log.Println("✓ EventSaver 종료 완료")

	// MongoDB 연결 종료
	mongoDB.Close()
	log.Println("✓ MongoDB 연결 종료 완료")

	log.Println("=== 서버 종료 완료 ===")
}

// handleWebSocket WebSocket 메시지 처리 (Goroutine)
func handleWebSocket(client *Client, audioHandler *webrtc.AudioHandler, roomRepo repository.RoomRepository, connManager *wsManager.ConnectionManager) {
	defer func() {
		client.Conn.Close()

		// ConnectionManager에서 연결 제거
		connManager.RemoveConnection(client.RoomID, client.ID)

		// Repository를 통해 클라이언트 제거
		ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()

		if err := roomRepo.RemoveClient(ctx, client.RoomID, client.ID); err != nil {
			log.Printf("클라이언트 제거 실패: %v", err)
		}

		log.Printf("✓ 클라이언트 연결 해제: Room=%s, Client=%s", client.RoomID, client.ID)
	}()

	for {
		var msg map[string]interface{}
		err := client.Conn.ReadJSON(&msg)
		if err != nil {
			if websocket.IsUnexpectedCloseError(err, websocket.CloseGoingAway, websocket.CloseAbnormalClosure) {
				log.Printf("WebSocket 오류: %v", err)
			}
			break
		}

		// 메시지 타입에 따라 처리
		msgType, ok := msg["type"].(string)
		if !ok {
			log.Println("잘못된 메시지 형식")
			continue
		}

		log.Printf("📨 수신 메시지: Room=%s, Client=%s, Type=%s", client.RoomID, client.ID, msgType)

		switch msgType {
		case "offer", "answer", "ice-candidate":
			// WebRTC 시그널링 메시지를 같은 룸의 다른 클라이언트에게 브로드캐스트
			connManager.BroadcastToRoom(client.RoomID, msg)

		default:
			log.Printf("알 수 없는 메시지 타입: %s", msgType)
		}
	}
}
