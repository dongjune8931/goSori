package webrtc

import (
	"io"
	"log"
	"time"

	"github.com/dongjune8931/goSori/internal/pipeline"
	"github.com/dongjune8931/goSori/pkg/models"
	"github.com/hraban/opus"
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
	decoder  *opus.Decoder
}

// NewAudioHandler 새로운 AudioHandler 인스턴스를 생성합니다
func NewAudioHandler(p *pipeline.AudioPipeline) *AudioHandler {
	// Opus 디코더 생성 (48kHz, 모노)
	decoder, err := opus.NewDecoder(OpusSampleRate, OpusChannels)
	if err != nil {
		log.Fatalf("Opus 디코더 생성 실패: %v", err)
	}

	return &AudioHandler{
		pipeline: p,
		decoder:  decoder,
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

		// Opus 데이터를 PCM으로 디코딩
		decodedAudio := h.decodeOpus(opusData)

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

// decodeOpus Opus 인코딩된 데이터를 PCM으로 디코딩합니다
func (h *AudioHandler) decodeOpus(opusData []byte) []byte {
	if len(opusData) == 0 {
		return nil
	}

	// PCM 버퍼 준비 (최대 5760 샘플, 120ms @ 48kHz)
	pcmBuffer := make([]int16, 5760)

	// Opus → PCM 디코딩
	n, err := h.decoder.Decode(opusData, pcmBuffer)
	if err != nil {
		log.Printf("❌ Opus 디코딩 실패: %v", err)
		return nil
	}

	// int16 슬라이스 → []byte 변환 (Little Endian)
	pcmBytes := make([]byte, n*2)
	for i := 0; i < n; i++ {
		sample := pcmBuffer[i]
		pcmBytes[i*2] = byte(sample)        // Low byte
		pcmBytes[i*2+1] = byte(sample >> 8) // High byte
	}

	return pcmBytes
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

// ProcessAudioData WebSocket을 통해 수신한 오디오 데이터를 처리합니다 (Public 메서드)
func (h *AudioHandler) ProcessAudioData(audioData []byte, userID, roomID string) error {
	if len(audioData) == 0 {
		return nil
	}

	// WebSocket에서 받은 데이터는 Opus 인코딩되어 있으므로 디코딩 필요
	pcmData := h.decodeOpus(audioData)
	if pcmData == nil {
		log.Printf("❌ WebSocket 오디오 디코딩 실패: UserID=%s, RoomID=%s", userID, roomID)
		return nil
	}

	// AudioChunk 생성 (디코딩된 PCM 데이터)
	chunk := models.NewAudioChunk(
		userID,
		roomID,
		pcmData,
		OpusSampleRate, // 48kHz
		OpusChannels,   // 모노
	)

	// 파이프라인에 제출
	if err := h.pipeline.ProcessAudio(chunk); err != nil {
		log.Printf("❌ WebSocket 오디오 청크 제출 실패: %v", err)
		return err
	}

	log.Printf("✅ WebSocket 오디오 청크 제출 성공: UserID=%s, RoomID=%s, Opus=%d bytes → PCM=%d bytes",
		userID, roomID, len(audioData), len(pcmData))
	return nil
}
