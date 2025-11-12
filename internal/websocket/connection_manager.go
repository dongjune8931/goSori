package websocket

import (
	"log"
	"sync"

	"github.com/dongjune8931/goSori/pkg/models"
	"github.com/gorilla/websocket"
)

// ConnectionManager WebSocket 연결을 관리합니다
type ConnectionManager struct {
	// roomID -> clientID -> *websocket.Conn
	rooms map[string]map[string]*websocket.Conn
	mu    sync.RWMutex
}

// NewConnectionManager 새로운 ConnectionManager를 생성합니다
func NewConnectionManager() *ConnectionManager {
	return &ConnectionManager{
		rooms: make(map[string]map[string]*websocket.Conn),
	}
}

// AddConnection 클라이언트 연결을 추가합니다
func (cm *ConnectionManager) AddConnection(roomID, clientID string, conn *websocket.Conn) {
	cm.mu.Lock()
	defer cm.mu.Unlock()

	if cm.rooms[roomID] == nil {
		cm.rooms[roomID] = make(map[string]*websocket.Conn)
	}
	cm.rooms[roomID][clientID] = conn

	log.Printf("✓ 연결 추가: Room=%s, Client=%s, 총 %d명", roomID, clientID, len(cm.rooms[roomID]))
}

// RemoveConnection 클라이언트 연결을 제거합니다
func (cm *ConnectionManager) RemoveConnection(roomID, clientID string) {
	cm.mu.Lock()
	defer cm.mu.Unlock()

	if cm.rooms[roomID] != nil {
		delete(cm.rooms[roomID], clientID)

		// 룸에 아무도 없으면 룸 삭제
		if len(cm.rooms[roomID]) == 0 {
			delete(cm.rooms, roomID)
			log.Printf("✓ 빈 룸 제거: Room=%s", roomID)
		} else {
			log.Printf("✓ 연결 제거: Room=%s, Client=%s, 남은 인원 %d명", roomID, clientID, len(cm.rooms[roomID]))
		}
	}
}

// BroadcastToRoom 룸의 모든 클라이언트에게 메시지를 브로드캐스트합니다
func (cm *ConnectionManager) BroadcastToRoom(roomID string, message interface{}) {
	cm.mu.RLock()
	clients := cm.rooms[roomID]
	cm.mu.RUnlock()

	if clients == nil {
		log.Printf("Warning: 룸을 찾을 수 없습니다: %s", roomID)
		return
	}

	var failedClients []string

	for clientID, conn := range clients {
		err := conn.WriteJSON(message)
		if err != nil {
			log.Printf("메시지 전송 실패: Room=%s, Client=%s, Error=%v", roomID, clientID, err)
			failedClients = append(failedClients, clientID)
		}
	}

	// 실패한 연결 제거
	for _, clientID := range failedClients {
		cm.RemoveConnection(roomID, clientID)
	}

	log.Printf("📤 브로드캐스트 완료: Room=%s, 수신자 %d명", roomID, len(clients)-len(failedClients))
}

// BroadcastTranslation 번역 결과를 브로드캐스트합니다
func (cm *ConnectionManager) BroadcastTranslation(event *models.TranslationEvent) {
	message := map[string]interface{}{
		"type":        "translation",
		"id":          event.ID,
		"user_id":     event.UserID,
		"room_id":     event.RoomID,
		"timestamp":   event.Timestamp,
		"source_text": event.SourceText,
		"source_lang": event.SourceLang,
		"target_text": event.TargetText,
		"target_lang": event.TargetLang,
	}

	cm.BroadcastToRoom(event.RoomID, message)
}

// GetRoomClients 룸의 클라이언트 수를 반환합니다
func (cm *ConnectionManager) GetRoomClients(roomID string) int {
	cm.mu.RLock()
	defer cm.mu.RUnlock()

	if cm.rooms[roomID] == nil {
		return 0
	}
	return len(cm.rooms[roomID])
}
