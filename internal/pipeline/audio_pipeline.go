package pipeline

import (
	"context"
	"log"
	"sync"

	"github.com/dongjune8931/goSori/pkg/models"
)

// AudioPipeline 전체 오디오 처리 파이프라인을 관리합니다
type AudioPipeline struct {
	sttPool         *STTWorkerPool
	translationPool *TranslationWorkerPool
	outputHandler   func(*models.TranslationEvent)
	ctx             context.Context
	cancel          context.CancelFunc
	wg              sync.WaitGroup
}

// NewAudioPipeline 새로운 AudioPipeline 인스턴스를 생성합니다
func NewAudioPipeline(
	sttPool *STTWorkerPool,
	translationPool *TranslationWorkerPool,
	outputHandler func(*models.TranslationEvent),
) *AudioPipeline {
	ctx, cancel := context.WithCancel(context.Background())

	return &AudioPipeline{
		sttPool:         sttPool,
		translationPool: translationPool,
		outputHandler:   outputHandler,
		ctx:             ctx,
		cancel:          cancel,
	}
}

// Start 전체 파이프라인을 시작합니다
func (p *AudioPipeline) Start() {
	log.Println("Audio Pipeline 시작")

	// STT 워커 풀 시작
	p.sttPool.Start()

	// Translation 워커 풀 시작
	p.translationPool.Start()

	// STT → Translation 연결 고루틴
	p.wg.Add(1)
	go p.connectSTTToTranslation()

	// Translation → Output Handler 연결 고루틴
	p.wg.Add(1)
	go p.connectTranslationToOutput()

	log.Println("Audio Pipeline 시작 완료")
}

// Stop 전체 파이프라인을 정상 종료합니다
func (p *AudioPipeline) Stop() {
	log.Println("Audio Pipeline 종료 시작...")

	// 컨텍스트 취소 (연결 고루틴 종료 시그널)
	p.cancel()

	// STT 워커 풀 종료
	p.sttPool.Stop()

	// Translation 워커 풀 종료
	p.translationPool.Stop()

	// 연결 고루틴 종료 대기
	p.wg.Wait()

	log.Println("Audio Pipeline 종료 완료")
}

// ProcessAudio 오디오 청크를 파이프라인에 입력합니다
func (p *AudioPipeline) ProcessAudio(chunk *models.AudioChunk) error {
	return p.sttPool.Submit(chunk)
}

// connectSTTToTranslation STT 출력을 Translation 입력으로 연결합니다
func (p *AudioPipeline) connectSTTToTranslation() {
	defer p.wg.Done()

	log.Println("STT → Translation 연결 고루틴 시작")

	for {
		select {
		case <-p.ctx.Done():
			log.Println("STT → Translation 연결 고루틴 종료")
			return

		case transcript, ok := <-p.sttPool.GetOutputQueue():
			if !ok {
				log.Println("STT 출력 큐 닫힘")
				return
			}

			// TranscriptEvent를 Translation Pool에 제출
			if err := p.translationPool.Submit(transcript); err != nil {
				log.Printf("Translation Pool 제출 실패: %v", err)
			}
		}
	}
}

// connectTranslationToOutput Translation 출력을 Output Handler로 연결합니다
func (p *AudioPipeline) connectTranslationToOutput() {
	defer p.wg.Done()

	log.Println("Translation → Output 연결 고루틴 시작")

	for {
		select {
		case <-p.ctx.Done():
			log.Println("Translation → Output 연결 고루틴 종료")
			return

		case translation, ok := <-p.translationPool.GetOutputQueue():
			if !ok {
				log.Println("Translation 출력 큐 닫힘")
				return
			}

			// Output Handler 호출
			if p.outputHandler != nil {
				p.outputHandler(translation)
			} else {
				log.Printf("Warning: outputHandler가 설정되지 않았습니다 (Translation ID: %s)", translation.ID)
			}
		}
	}
}
