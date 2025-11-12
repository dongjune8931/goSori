package handlers

import (
	"context"
	"io"
	"log"
	"net/http"
	"sync"
	"time"

	"github.com/dongjune8931/goSori/internal/pipeline"
	"github.com/dongjune8931/goSori/pkg/models"
	"github.com/gin-gonic/gin"
)

// AudioUploadHandler 오디오 파일 업로드 및 처리 핸들러
type AudioUploadHandler struct {
	pipeline *pipeline.AudioPipeline
	results  map[string]*TranslationResult
	mu       sync.RWMutex
}

// TranslationResult 번역 결과
type TranslationResult struct {
	SourceText string `json:"source_text"`
	SourceLang string `json:"source_lang"`
	TargetText string `json:"target_text"`
	TargetLang string `json:"target_lang"`
	Timestamp  int64  `json:"timestamp"`
}

// NewAudioUploadHandler 새로운 핸들러 생성
func NewAudioUploadHandler(p *pipeline.AudioPipeline) *AudioUploadHandler {
	return &AudioUploadHandler{
		pipeline: p,
		results:  make(map[string]*TranslationResult),
	}
}

// SaveResult 번역 결과 저장
func (h *AudioUploadHandler) SaveResult(event *models.TranslationEvent) {
	h.mu.Lock()
	defer h.mu.Unlock()

	h.results[event.ID] = &TranslationResult{
		SourceText: event.SourceText,
		SourceLang: event.SourceLang,
		TargetText: event.TargetText,
		TargetLang: event.TargetLang,
		Timestamp:  event.Timestamp.Unix(),
	}
}

// UploadAudio 오디오 파일 업로드 및 처리
func (h *AudioUploadHandler) UploadAudio(c *gin.Context) {
	// 멀티파트 폼에서 파일 읽기
	file, header, err := c.Request.FormFile("audio")
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "오디오 파일이 필요합니다"})
		return
	}
	defer file.Close()

	log.Printf("오디오 파일 업로드: %s (%d bytes)", header.Filename, header.Size)

	// 파일 데이터 읽기
	audioData, err := io.ReadAll(file)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "파일 읽기 실패"})
		return
	}

	// AudioChunk 생성
	userID := c.DefaultQuery("userId", "test-user")
	roomID := c.DefaultQuery("roomId", "test-room")

	chunk := models.NewAudioChunk(
		userID,
		roomID,
		audioData,
		16000, // 16kHz
		1,     // Mono
	)

	// 파이프라인에 전송
	if err := h.pipeline.ProcessAudio(chunk); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	log.Printf("AudioChunk 전송 완료: %s", chunk.ID)

	// 결과 대기 (최대 30초)
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	// 결과를 폴링 방식으로 확인
	ticker := time.NewTicker(500 * time.Millisecond)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			c.JSON(http.StatusRequestTimeout, gin.H{
				"error":    "처리 시간 초과",
				"chunk_id": chunk.ID,
				"message":  "파이프라인 처리 중입니다. 나중에 GET /test/result/:chunkId로 결과를 확인하세요.",
			})
			return

		case <-ticker.C:
			// 결과 확인
			h.mu.RLock()
			result := h.findResultByChunkID(chunk.ID)
			h.mu.RUnlock()

			if result != nil {
				c.JSON(http.StatusOK, gin.H{
					"chunk_id": chunk.ID,
					"result":   result,
				})
				return
			}
		}
	}
}

// findResultByChunkID AudioChunkID와 연결된 TranslationResult 찾기
func (h *AudioUploadHandler) findResultByChunkID(chunkID string) *TranslationResult {
	// TranscriptEvent의 AudioChunkID와 TranslationEvent의 TranscriptID를 추적해야 함
	// 간단한 구현: 최근 결과 반환 (실제로는 더 정교한 매핑 필요)
	if len(h.results) > 0 {
		// 가장 최근 결과 반환
		var latest *TranslationResult
		var latestTime int64
		for _, result := range h.results {
			if result.Timestamp > latestTime {
				latest = result
				latestTime = result.Timestamp
			}
		}
		return latest
	}
	return nil
}

// GetResult 결과 조회
func (h *AudioUploadHandler) GetResult(c *gin.Context) {
	resultID := c.Param("id")

	h.mu.RLock()
	result, exists := h.results[resultID]
	h.mu.RUnlock()

	if !exists {
		c.JSON(http.StatusNotFound, gin.H{"error": "결과를 찾을 수 없습니다"})
		return
	}

	c.JSON(http.StatusOK, result)
}
