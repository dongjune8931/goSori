package pipeline

import (
	"context"
	"log"
	"sync"
	"time"

	"github.com/dongjune8931/goSori/pkg/models"
	"github.com/dongjune8931/goSori/pkg/repository"
)

// EventSaver Channel 기반 비동기 이벤트 저장
// Go의 Goroutine & Channel 활용:
// - Non-blocking: 파이프라인 처리 속도에 영향 없음
// - Concurrent: 여러 이벤트를 동시에 저장
// - Buffered Channel: 일시적인 burst 처리
type EventSaver struct {
	eventRepo repository.EventRepository

	// Go Channel: 이벤트 스트림
	audioChunkChan  chan *models.AudioChunk
	transcriptChan  chan *models.TranscriptEvent
	translationChan chan *models.TranslationEvent

	// Context: Graceful shutdown
	ctx    context.Context
	cancel context.CancelFunc

	// WaitGroup: 모든 고루틴 종료 대기
	wg sync.WaitGroup
}

// NewEventSaver 새로운 EventSaver를 생성합니다
func NewEventSaver(eventRepo repository.EventRepository, bufferSize int) *EventSaver {
	ctx, cancel := context.WithCancel(context.Background())

	return &EventSaver{
		eventRepo:       eventRepo,
		audioChunkChan:  make(chan *models.AudioChunk, bufferSize),
		transcriptChan:  make(chan *models.TranscriptEvent, bufferSize),
		translationChan: make(chan *models.TranslationEvent, bufferSize),
		ctx:             ctx,
		cancel:          cancel,
	}
}

// Start 이벤트 저장 워커들을 시작합니다
func (s *EventSaver) Start() {
	log.Println("EventSaver 시작")

	// AudioChunk 저장 워커
	s.wg.Add(1)
	go s.audioChunkWorker()

	// Transcript 저장 워커
	s.wg.Add(1)
	go s.transcriptWorker()

	// Translation 저장 워커
	s.wg.Add(1)
	go s.translationWorker()
}

// Stop EventSaver를 정상 종료합니다
func (s *EventSaver) Stop() {
	log.Println("EventSaver 종료 시작...")

	// Context 취소
	s.cancel()

	// 모든 워커 종료 대기
	s.wg.Wait()

	// 채널 닫기
	close(s.audioChunkChan)
	close(s.transcriptChan)
	close(s.translationChan)

	log.Println("EventSaver 종료 완료")
}

// SaveAudioChunk AudioChunk를 비동기로 저장합니다 (Non-blocking)
func (s *EventSaver) SaveAudioChunk(chunk *models.AudioChunk) {
	select {
	case s.audioChunkChan <- chunk:
		// 성공
	case <-s.ctx.Done():
		log.Println("EventSaver 종료됨, AudioChunk 저장 실패")
	}
}

// SaveTranscript Transcript를 비동기로 저장합니다
func (s *EventSaver) SaveTranscript(transcript *models.TranscriptEvent) {
	select {
	case s.transcriptChan <- transcript:
		// 성공
	case <-s.ctx.Done():
		log.Println("EventSaver 종료됨, Transcript 저장 실패")
	}
}

// SaveTranslation Translation을 비동기로 저장합니다
func (s *EventSaver) SaveTranslation(translation *models.TranslationEvent) {
	select {
	case s.translationChan <- translation:
		// 성공
	case <-s.ctx.Done():
		log.Println("EventSaver 종료됨, Translation 저장 실패")
	}
}

// audioChunkWorker AudioChunk 저장 워커 (Goroutine)
func (s *EventSaver) audioChunkWorker() {
	defer s.wg.Done()

	for {
		select {
		case <-s.ctx.Done():
			log.Println("AudioChunk 워커 종료")
			return

		case chunk, ok := <-s.audioChunkChan:
			if !ok {
				return
			}

			// Context with timeout for MongoDB operation
			ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
			if err := s.eventRepo.SaveAudioChunk(ctx, chunk); err != nil {
				log.Printf("AudioChunk 저장 실패: %v", err)
			}
			cancel()
		}
	}
}

// transcriptWorker Transcript 저장 워커 (Goroutine)
func (s *EventSaver) transcriptWorker() {
	defer s.wg.Done()

	for {
		select {
		case <-s.ctx.Done():
			log.Println("Transcript 워커 종료")
			return

		case transcript, ok := <-s.transcriptChan:
			if !ok {
				return
			}

			// Context with timeout
			ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
			if err := s.eventRepo.SaveTranscript(ctx, transcript); err != nil {
				log.Printf("Transcript 저장 실패: %v", err)
			}
			cancel()
		}
	}
}

// translationWorker Translation 저장 워커 (Goroutine)
func (s *EventSaver) translationWorker() {
	defer s.wg.Done()

	for {
		select {
		case <-s.ctx.Done():
			log.Println("Translation 워커 종료")
			return

		case translation, ok := <-s.translationChan:
			if !ok {
				return
			}

			// Context with timeout
			ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
			if err := s.eventRepo.SaveTranslation(ctx, translation); err != nil {
				log.Printf("Translation 저장 실패: %v", err)
			}
			cancel()
		}
	}
}
