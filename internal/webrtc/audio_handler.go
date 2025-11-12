package webrtc

import (
	"io"
	"log"
	"time"

	"github.com/dongjune8931/goSori/internal/pipeline"
	"github.com/dongjune8931/goSori/pkg/models"
	"github.com/pion/webrtc/v3"
)

const (
	// OpusFrameDuration Opus 오디오 프레임 지속 시간 (20ms)
	OpusFrameDuration = 20 * time.Millisecond
	// ChunkDuration 오디오 청크 지속 시간 (100ms)
	ChunkDuration = 100 * time.Millisecond
	// FramesPerChunk 청크당 프레임 수 (100ms / 20ms = 5)
	FramesPerChunk = int(ChunkDuration / OpusFrameDuration)
	// OpusSampleRate Opus 샘플 레이트 (48kHz)
	OpusSampleRate = 48000
	// OpusChannels Opus 채널 수 (모노)
	OpusChannels = 1
)

// AudioHandler WebRTC 오디오 트랙을 처리합니다
type AudioHandler struct {
	pipeline *pipeline.AudioPipeline
}

// NewAudioHandler 새로운 AudioHandler 인스턴스를 생성합니다
func NewAudioHandler(p *pipeline.AudioPipeline) *AudioHandler {
	return &AudioHandler{
		pipeline: p,
	}
}

// OnTrack WebRTC 트랙이 수신될 때 호출되는 콜백입니다
func (h *AudioHandler) OnTrack(track *webrtc.TrackRemote, receiver *webrtc.RTPReceiver, userID, roomID string) {
	log.Printf("새로운 오디오 트랙 수신: UserID=%s, RoomID=%s, Kind=%s", userID, roomID, track.Kind())

	if track.Kind() != webrtc.RTPCodecTypeAudio {
		log.Printf("비-오디오 트랙 무시: %s", track.Kind())
		return
	}

	// 고루틴으로 오디오 스트림 처리
	go h.processAudioTrack(track, userID, roomID)
}

// processAudioTrack 오디오 트랙에서 RTP 패킷을 읽고 처리합니다
func (h *AudioHandler) processAudioTrack(track *webrtc.TrackRemote, userID, roomID string) {
	log.Printf("오디오 트랙 처리 시작: UserID=%s, RoomID=%s", userID, roomID)

	// 오디오 데이터 버퍼 (100ms 단위로 모음)
	var audioBuffer []byte
	frameCount := 0

	for {
		// RTP 패킷 읽기
		rtpPacket, _, err := track.ReadRTP()
		if err != nil {
			if err == io.EOF {
				log.Printf("오디오 트랙 종료: UserID=%s, RoomID=%s", userID, roomID)
			} else {
				log.Printf("RTP 패킷 읽기 오류: %v", err)
			}
			break
		}

		// RTP 페이로드 추출 (Opus 인코딩된 오디오 데이터)
		opusData := rtpPacket.Payload

		// TODO: 실제 프로덕션에서는 Opus 디코더를 사용해야 함
		// 현재는 시뮬레이션: Opus 데이터를 그대로 사용 (또는 PCM으로 변환)
		decodedAudio := h.simulateOpusDecoding(opusData)

		// 버퍼에 추가
		audioBuffer = append(audioBuffer, decodedAudio...)
		frameCount++

		// 100ms (5개 프레임) 모였으면 청크 생성 및 전송
		if frameCount >= FramesPerChunk {
			h.sendAudioChunk(audioBuffer, userID, roomID)

			// 버퍼 초기화
			audioBuffer = nil
			frameCount = 0
		}
	}

	// 마지막 남은 데이터 처리
	if len(audioBuffer) > 0 {
		h.sendAudioChunk(audioBuffer, userID, roomID)
	}

	log.Printf("오디오 트랙 처리 완료: UserID=%s, RoomID=%s", userID, roomID)
}

// simulateOpusDecoding Opus 디코딩을 시뮬레이션합니다
// TODO: 실제 프로덕션에서는 github.com/pion/opus 또는 gopus 사용
func (h *AudioHandler) simulateOpusDecoding(opusData []byte) []byte {
	// 시뮬레이션: Opus 데이터를 그대로 반환
	// 실제로는 PCM 데이터로 디코딩해야 함
	// 예: Opus -> PCM 16-bit, 48kHz, Mono
	return opusData
}

// sendAudioChunk 오디오 청크를 파이프라인에 전송합니다
func (h *AudioHandler) sendAudioChunk(audioData []byte, userID, roomID string) {
	if len(audioData) == 0 {
		return
	}

	// AudioChunk 생성
	chunk := models.NewAudioChunk(
		userID,
		roomID,
		audioData,
		OpusSampleRate,
		OpusChannels,
	)

	// 파이프라인에 제출
	if err := h.pipeline.ProcessAudio(chunk); err != nil {
		log.Printf("AudioChunk 제출 실패: %v", err)
	} else {
		log.Printf("AudioChunk 제출 성공: UserID=%s, RoomID=%s, Size=%d bytes", userID, roomID, len(audioData))
	}
}
