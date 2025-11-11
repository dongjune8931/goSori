package pipeline

import (
	"context"
	"fmt"
	"log"
	"sync"

	"github.com/dongjune8931/goSori/internal/ai"
	"github.com/dongjune8931/goSori/pkg/models"
)

// STTWorkerPool STT 작업을 병렬로 처리하는 워커 풀
type STTWorkerPool struct {
	workers     int
	inputQueue  chan *models.AudioChunk
	outputQueue chan *models.TranscriptEvent
	sttClient   *ai.STTClient
	ctx         context.Context
	cancel      context.CancelFunc
	wg          sync.WaitGroup
}

// NewSTTWorkerPool 새로운 STTWorkerPool 인스턴스를 생성합니다
func NewSTTWorkerPool(
	workers int,
	inputQueueSize int,
	outputQueueSize int,
	sttClient *ai.STTClient,
) *STTWorkerPool {
	ctx, cancel := context.WithCancel(context.Background())

	return &STTWorkerPool{
		workers:     workers,
		inputQueue:  make(chan *models.AudioChunk, inputQueueSize),
		outputQueue: make(chan *models.TranscriptEvent, outputQueueSize),
		sttClient:   sttClient,
		ctx:         ctx,
		cancel:      cancel,
	}
}

// Start 워커들을 시작합니다
func (p *STTWorkerPool) Start() {
	log.Printf("STT Worker Pool 시작: %d개 워커", p.workers)

	for i := 0; i < p.workers; i++ {
		p.wg.Add(1)
		go p.worker(i)
	}
}

// Stop 워커 풀을 정상 종료합니다
func (p *STTWorkerPool) Stop() {
	log.Println("STT Worker Pool 종료 시작...")

	// 컨텍스트 취소 시그널 전송
	p.cancel()

	// 모든 워커가 종료될 때까지 대기
	p.wg.Wait()

	// 큐 닫기
	close(p.inputQueue)
	close(p.outputQueue)

	log.Println("STT Worker Pool 종료 완료")
}

// Submit 오디오 청크를 처리 큐에 제출합니다
func (p *STTWorkerPool) Submit(chunk *models.AudioChunk) error {
	select {
	case p.inputQueue <- chunk:
		return nil
	case <-p.ctx.Done():
		return fmt.Errorf("워커 풀이 종료되었습니다")
	}
}

// GetOutputQueue 출력 큐를 반환합니다 (다음 파이프라인 단계에서 사용)
func (p *STTWorkerPool) GetOutputQueue() <-chan *models.TranscriptEvent {
	return p.outputQueue
}

// worker 개별 워커 고루틴
func (p *STTWorkerPool) worker(id int) {
	defer p.wg.Done()

	log.Printf("STT Worker #%d 시작", id)

	for {
		select {
		case <-p.ctx.Done():
			log.Printf("STT Worker #%d 종료", id)
			return

		case chunk, ok := <-p.inputQueue:
			if !ok {
				log.Printf("STT Worker #%d: 입력 큐 닫힘", id)
				return
			}

			// STT 처리
			text, err := p.sttClient.Transcribe(chunk.Data)
			if err != nil {
				log.Printf("STT Worker #%d: 변환 실패 (Chunk: %s): %v", id, chunk.ID, err)
				continue
			}

			// TranscriptEvent 생성
			event := models.NewTranscriptEvent(
				chunk.ID,
				chunk.UserID,
				chunk.RoomID,
				text,
				"auto", // 언어 자동 감지
				1.0,    // Whisper는 confidence를 제공하지 않으므로 기본값
			)

			// 출력 큐로 전송
			select {
			case p.outputQueue <- event:
				log.Printf("STT Worker #%d: 변환 완료 (Chunk: %s, Text: %s)", id, chunk.ID, text)
			case <-p.ctx.Done():
				log.Printf("STT Worker #%d 종료", id)
				return
			}
		}
	}
}
