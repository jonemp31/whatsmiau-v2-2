package whatsmiau

import (
	"context"
	"io"
	"strconv"
	"strings"
	"time"

	"go.mau.fi/whatsmeow"
	"go.mau.fi/whatsmeow/proto/waE2E"
	"go.mau.fi/whatsmeow/types"
	"go.uber.org/zap"
	"google.golang.org/protobuf/proto"
)

type SendStatusTextRequest struct {
	InstanceID string `json:"instance_id"`
	Text       string `json:"text"`
	Background string `json:"background"`
	Font       string `json:"font"`
}

type SendStatusImageRequest struct {
	InstanceID string `json:"instance_id"`
	MediaURL   string `json:"media_url"`
	Caption    string `json:"caption"`
	Mimetype   string `json:"mimetype"`
}

type SendStatusVideoRequest struct {
	InstanceID string `json:"instance_id"`
	MediaURL   string `json:"media_url"`
	Caption    string `json:"caption"`
	Mimetype   string `json:"mimetype"`
}

type SendStatusAudioRequest struct {
	InstanceID string `json:"instance_id"`
	MediaURL   string `json:"media_url"`
	Mimetype   string `json:"mimetype"`
}

type SendStatusResponse struct {
	ID        string    `json:"id"`
	CreatedAt time.Time `json:"created_at"`
}

// SendStatusText envia status de texto
func (s *Whatsmiau) SendStatusText(ctx context.Context, data *SendStatusTextRequest) (*SendStatusResponse, error) {
	client, ok := s.clients.Load(data.InstanceID)
	if !ok {
		return nil, whatsmeow.ErrClientIsNil
	}

	// Converter cor de fundo para ARGB
	backgroundArgb := parseBackgroundColor(data.Background)

	// Criar status de texto
	status := &waE2E.Message{
		ExtendedTextMessage: &waE2E.ExtendedTextMessage{
			Text:           proto.String(data.Text),
			BackgroundArgb: proto.Uint32(backgroundArgb),
		},
	}

	// Enviar para status (broadcast para todos os contatos)
	res, err := client.SendMessage(ctx, types.StatusBroadcastJID, status)
	if err != nil {
		return nil, err
	}

	return &SendStatusResponse{
		ID:        res.ID,
		CreatedAt: res.Timestamp,
	}, nil
}

// SendStatusImage envia status de imagem
func (s *Whatsmiau) SendStatusImage(ctx context.Context, data *SendStatusImageRequest) (*SendStatusResponse, error) {
	client, ok := s.clients.Load(data.InstanceID)
	if !ok {
		return nil, whatsmeow.ErrClientIsNil
	}

	// Baixar e fazer upload da imagem
	resMedia, err := s.httpClient.Get(data.MediaURL)
	if err != nil {
		return nil, err
	}

	dataBytes, err := io.ReadAll(resMedia.Body)
	if err != nil {
		return nil, err
	}

	uploaded, err := client.Upload(ctx, dataBytes, whatsmeow.MediaImage)
	if err != nil {
		return nil, err
	}

	// Detectar MIME type se não especificado
	if data.Mimetype == "" {
		data.Mimetype, err = extractMimetype(dataBytes, uploaded.URL)
		if err != nil {
			zap.L().Warn("failed to extract mimetype for status image", zap.Error(err))
			data.Mimetype = "image/jpeg" // Fallback
		}
	}

	// Criar status de imagem
	status := &waE2E.Message{
		ImageMessage: &waE2E.ImageMessage{
			URL:               proto.String(uploaded.URL),
			Mimetype:          proto.String(data.Mimetype),
			Caption:           proto.String(data.Caption),
			FileSHA256:        uploaded.FileSHA256,
			FileLength:        proto.Uint64(uploaded.FileLength),
			MediaKey:          uploaded.MediaKey,
			FileEncSHA256:     uploaded.FileEncSHA256,
			DirectPath:        proto.String(uploaded.DirectPath),
			MediaKeyTimestamp: proto.Int64(0),
		},
	}

	// Enviar para status
	res, err := client.SendMessage(ctx, types.StatusBroadcastJID, status)
	if err != nil {
		return nil, err
	}

	return &SendStatusResponse{
		ID:        res.ID,
		CreatedAt: res.Timestamp,
	}, nil
}

// SendStatusVideo envia status de vídeo
func (s *Whatsmiau) SendStatusVideo(ctx context.Context, data *SendStatusVideoRequest) (*SendStatusResponse, error) {
	client, ok := s.clients.Load(data.InstanceID)
	if !ok {
		return nil, whatsmeow.ErrClientIsNil
	}

	// Baixar e fazer upload do vídeo
	resMedia, err := s.httpClient.Get(data.MediaURL)
	if err != nil {
		return nil, err
	}

	dataBytes, err := io.ReadAll(resMedia.Body)
	if err != nil {
		return nil, err
	}

	uploaded, err := client.Upload(ctx, dataBytes, whatsmeow.MediaVideo)
	if err != nil {
		return nil, err
	}

	// Detectar MIME type se não especificado
	if data.Mimetype == "" {
		data.Mimetype, err = extractMimetype(dataBytes, uploaded.URL)
		if err != nil {
			zap.L().Warn("failed to extract mimetype for status video", zap.Error(err))
			data.Mimetype = "video/mp4" // Fallback
		}
	}

	// Criar status de vídeo
	status := &waE2E.Message{
		VideoMessage: &waE2E.VideoMessage{
			URL:               proto.String(uploaded.URL),
			Mimetype:          proto.String(data.Mimetype),
			Caption:           proto.String(data.Caption),
			FileSHA256:        uploaded.FileSHA256,
			FileLength:        proto.Uint64(uploaded.FileLength),
			MediaKey:          uploaded.MediaKey,
			FileEncSHA256:     uploaded.FileEncSHA256,
			DirectPath:        proto.String(uploaded.DirectPath),
			MediaKeyTimestamp: proto.Int64(0),
		},
	}

	// Enviar para status
	res, err := client.SendMessage(ctx, types.StatusBroadcastJID, status)
	if err != nil {
		return nil, err
	}

	return &SendStatusResponse{
		ID:        res.ID,
		CreatedAt: res.Timestamp,
	}, nil
}

// SendStatusAudio envia status de áudio
func (s *Whatsmiau) SendStatusAudio(ctx context.Context, data *SendStatusAudioRequest) (*SendStatusResponse, error) {
	client, ok := s.clients.Load(data.InstanceID)
	if !ok {
		return nil, whatsmeow.ErrClientIsNil
	}

	// Usar o converter service para processar áudio
	audioData, err := s.converter.ProcessMedia(ctx, data.MediaURL, "audio")
	if err != nil {
		return nil, err
	}
	
	// Para manter compatibilidade, usar valores padrão
	waveForm := []byte{}
	secs := 0.0

	uploaded, err := client.Upload(ctx, audioData, whatsmeow.MediaAudio)
	if err != nil {
		return nil, err
	}

	// Criar status de áudio
	status := &waE2E.Message{
		AudioMessage: &waE2E.AudioMessage{
			URL:               proto.String(uploaded.URL),
			Mimetype:          proto.String("audio/ogg; codecs=opus"),
			FileSHA256:        uploaded.FileSHA256,
			FileLength:        proto.Uint64(uploaded.FileLength),
			Seconds:           proto.Uint32(uint32(secs)),
			PTT:               proto.Bool(true),
			MediaKey:          uploaded.MediaKey,
			FileEncSHA256:     uploaded.FileEncSHA256,
			DirectPath:        proto.String(uploaded.DirectPath),
			Waveform:          waveForm,
			MediaKeyTimestamp: proto.Int64(0),
		},
	}

	// Enviar para status
	res, err := client.SendMessage(ctx, types.StatusBroadcastJID, status)
	if err != nil {
		return nil, err
	}

	return &SendStatusResponse{
		ID:        res.ID,
		CreatedAt: res.Timestamp,
	}, nil
}

// parseBackgroundColor converte cor hexadecimal para ARGB
func parseBackgroundColor(hex string) uint32 {
	if hex == "" {
		return 0xFFFFFFFF // Branco padrão
	}

	// Remover # se presente
	hex = strings.TrimPrefix(hex, "#")

	// Adicionar alpha se não presente (assumir FF para opacidade total)
	if len(hex) == 6 {
		hex = "FF" + hex
	}

	// Converter para uint32
	if val, err := strconv.ParseUint(hex, 16, 32); err == nil {
		return uint32(val)
	}

	return 0xFFFFFFFF // Branco padrão em caso de erro
}
