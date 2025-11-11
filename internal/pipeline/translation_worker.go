package pipeline

import (
	"context"
	"fmt"
	"log"
	"sync"

	"github.com/dongjune8931/goSori/internal/ai"
	"github.com/dongjune8931/goSori/pkg/models"
)

// TranslationWorkerPool 번역 작업을 병렬로 처리하는 워커 풀
type TranslationWorkerPool struct {
	workers           int
	inputQueue        chan *models.TranscriptEvent
	outputQueue       chan *models.TranslationEvent
	translationClient *ai.TranslationClient
	ctx               context.Context
	cancel            context.CancelFunc
	wg                sync.WaitGroup
}

// NewTranslationWorkerPool 새로운 TranslationWorkerPool 인스턴스를 생성합니다
func NewTranslationWorkerPool(
	workers int,
	inputQueueSize int,
	outputQueueSize int,
	translationClient *ai.TranslationClient,
) *TranslationWorkerPool {
	ctx, cancel := context.WithCancel(context.Background())

	return &TranslationWorkerPool{
		workers:           workers,
		inputQueue:        make(chan *models.TranscriptEvent, inputQueueSize),
		outputQueue:       make(chan *models.TranslationEvent, outputQueueSize),
		translationClient: translationClient,
		ctx:               ctx,
		cancel:            cancel,
	}
}

// Start 워커들을 시작합니다
func (p *TranslationWorkerPool) Start() {
	log.Printf("Translation Worker Pool 시작: %d개 워커", p.workers)

	for i := 0; i < p.workers; i++ {
		p.wg.Add(1)
		go p.worker(i)
	}
}

// Stop 워커 풀을 정상 종료합니다
func (p *TranslationWorkerPool) Stop() {
	log.Println("Translation Worker Pool 종료 시작...")

	// 컨텍스트 취소 시그널 전송
	p.cancel()

	// 모든 워커가 종료될 때까지 대기
	p.wg.Wait()

	// 큐 닫기
	close(p.inputQueue)
	close(p.outputQueue)

	log.Println("Translation Worker Pool 종료 완료")
}

// Submit 변환된 텍스트를 번역 큐에 제출합니다
func (p *TranslationWorkerPool) Submit(event *models.TranscriptEvent) error {
	select {
	case p.inputQueue <- event:
		return nil
	case <-p.ctx.Done():
		return fmt.Errorf("워커 풀이 종료되었습니다")
	}
}

// GetOutputQueue 출력 큐를 반환합니다 (다음 파이프라인 단계에서 사용)
func (p *TranslationWorkerPool) GetOutputQueue() <-chan *models.TranslationEvent {
	return p.outputQueue
}

// worker 개별 워커 고루틴
func (p *TranslationWorkerPool) worker(id int) {
	defer p.wg.Done()

	log.Printf("Translation Worker #%d 시작", id)

	for {
		select {
		case <-p.ctx.Done():
			log.Printf("Translation Worker #%d 종료", id)
			return

		case transcript, ok := <-p.inputQueue:
			if !ok {
				log.Printf("Translation Worker #%d: 입력 큐 닫힘", id)
				return
			}

			// 타겟 언어 결정 (TODO: 사용자별 설정으로 개선 필요)
			targetLang := p.determineTargetLanguage(transcript.Language)

			// 번역 수행
			translatedText, err := p.translationClient.Translate(
				transcript.Text,
				transcript.Language,
				targetLang,
			)
			if err != nil {
				log.Printf("Translation Worker #%d: 번역 실패 (Transcript: %s): %v", id, transcript.ID, err)
				continue
			}

			// TranslationEvent 생성
			event := models.NewTranslationEvent(
				transcript.ID,
				transcript.UserID,
				transcript.RoomID,
				transcript.Text,
				transcript.Language,
				translatedText,
				targetLang,
			)

			// 출력 큐로 전송
			select {
			case p.outputQueue <- event:
				log.Printf("Translation Worker #%d: 번역 완료 (Transcript: %s, %s→%s)",
					id, transcript.ID, transcript.Language, targetLang)
			case <-p.ctx.Done():
				log.Printf("Translation Worker #%d 종료", id)
				return
			}
		}
	}
}

// determineTargetLanguage 소스 언어를 기반으로 타겟 언어를 결정합니다
// TODO: 사용자별 설정 또는 룸별 설정으로 개선 필요
func (p *TranslationWorkerPool) determineTargetLanguage(sourceLang string) string {
	// 간단한 로직: 한국어면 영어로, 그 외는 한국어로
	switch sourceLang {
	case "Korean", "ko", "한국어":
		return "English"
	default:
		return "Korean"
	}
}
