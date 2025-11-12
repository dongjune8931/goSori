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
	"github.com/dongjune8931/goSori/internal/pipeline"
	"github.com/dongjune8931/goSori/internal/webrtc"
	"github.com/dongjune8931/goSori/pkg/config"
	"github.com/dongjune8931/goSori/pkg/models"
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

// RoomManager 간단한 룸 관리 (메모리 기반)
type RoomManager struct {
	rooms map[string]*Room
}

// Room WebRTC 룸
type Room struct {
	ID      string
	Clients map[string]*Client
}

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

	// 2. AI 클라이언트 생성
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

	// 3. 워커 풀 생성
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

	// 4. AudioPipeline 생성 및 시작
	audioPipeline := pipeline.NewAudioPipeline(
		sttPool,
		translationPool,
		func(event *models.TranslationEvent) {
			// Output Handler: 번역 결과 처리
			log.Printf("📝 번역 완료: [%s→%s] %s → %s",
				event.SourceLang, event.TargetLang,
				event.SourceText, event.TargetText)
			// TODO: WebSocket으로 클라이언트에 전송
		},
	)
	audioPipeline.Start()
	log.Println("✓ Audio Pipeline 시작 완료")

	// 5. AudioHandler 생성
	audioHandler := webrtc.NewAudioHandler(audioPipeline)
	log.Println("✓ Audio Handler 생성 완료")

	// 6. RoomManager 생성
	roomManager := &RoomManager{
		rooms: make(map[string]*Room),
	}
	log.Println("✓ Room Manager 생성 완료")

	// 7. Gin HTTP 서버 설정
	gin.SetMode(gin.ReleaseMode)
	router := gin.Default()

	// Health Check
	router.GET("/health", func(c *gin.Context) {
		c.JSON(http.StatusOK, gin.H{
			"status": "healthy",
			"time":   time.Now(),
		})
	})

	// 룸 생성
	router.POST("/room", func(c *gin.Context) {
		roomID := fmt.Sprintf("room-%d", time.Now().Unix())
		room := &Room{
			ID:      roomID,
			Clients: make(map[string]*Client),
		}
		roomManager.rooms[roomID] = room

		log.Printf("✓ 룸 생성: %s", roomID)

		c.JSON(http.StatusCreated, gin.H{
			"room_id": roomID,
		})
	})

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

		// 클라이언트 등록
		client := &Client{
			ID:     clientID,
			RoomID: roomID,
			Conn:   conn,
		}

		room, exists := roomManager.rooms[roomID]
		if !exists {
			room = &Room{
				ID:      roomID,
				Clients: make(map[string]*Client),
			}
			roomManager.rooms[roomID] = room
		}
		room.Clients[clientID] = client

		log.Printf("✓ 클라이언트 연결: Room=%s, Client=%s", roomID, clientID)

		// WebSocket 메시지 처리
		go handleWebSocket(client, audioHandler, roomManager)
	})

	// 8. HTTP 서버 시작
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

	// 9. Graceful Shutdown
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

	log.Println("=== 서버 종료 완료 ===")
}

// handleWebSocket WebSocket 메시지 처리
func handleWebSocket(client *Client, audioHandler *webrtc.AudioHandler, roomManager *RoomManager) {
	defer func() {
		client.Conn.Close()

		// 룸에서 클라이언트 제거
		if room, exists := roomManager.rooms[client.RoomID]; exists {
			delete(room.Clients, client.ID)
			log.Printf("✓ 클라이언트 연결 해제: Room=%s, Client=%s", client.RoomID, client.ID)
		}
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
			broadcastToRoom(client, roomManager, msg)

		default:
			log.Printf("알 수 없는 메시지 타입: %s", msgType)
		}
	}
}

// broadcastToRoom 같은 룸의 다른 클라이언트에게 메시지 브로드캐스트
func broadcastToRoom(sender *Client, roomManager *RoomManager, msg map[string]interface{}) {
	room, exists := roomManager.rooms[sender.RoomID]
	if !exists {
		return
	}

	for clientID, client := range room.Clients {
		if clientID == sender.ID {
			continue // 보낸 사람 제외
		}

		err := client.Conn.WriteJSON(msg)
		if err != nil {
			log.Printf("메시지 전송 실패: %v", err)
		}
	}
}
