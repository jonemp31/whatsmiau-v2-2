package converter

import (
	"bytes"
	"context"
	"fmt"
	"io"
	"net/http"
	"os/exec"
	"strconv"
	"strings"
	"time"

	"go.uber.org/zap"
)

// ConverterService gerencia todo o processo de conversão de mídia.
type ConverterService struct {
	workerPool *WorkerPool
	httpClient *http.Client
	shutdown   func()
}

// NewConverterService inicializa o serviço de conversão e seus componentes.
func NewConverterService(workerCount int, httpTimeout time.Duration) *ConverterService {
	workerPool := NewWorkerPool(workerCount)
	if err := workerPool.Start(); err != nil {
		zap.L().Fatal("failed to start converter worker pool", zap.Error(err))
	}

	httpClient := &http.Client{
		Timeout: httpTimeout,
		Transport: &http.Transport{
			MaxIdleConns:        100,
			MaxIdleConnsPerHost: 10,
			IdleConnTimeout:     90 * time.Second,
		},
	}

	shutdown := func() {
		workerPool.Stop()
		httpClient.CloseIdleConnections()
		zap.L().Info("Converter service shut down gracefully")
	}

	return &ConverterService{
		workerPool: workerPool,
		httpClient: httpClient,
		shutdown:   shutdown,
	}
}

// Shutdown para a execução do serviço de conversão.
func (s *ConverterService) Shutdown() {
	s.shutdown()
}

// ProcessMedia baixa um arquivo de uma URL e o converte para o formato desejado.
func (s *ConverterService) ProcessMedia(ctx context.Context, url string, mediaType string) ([]byte, error) {
	// 1. Baixar o arquivo
	originalData, err := s.download(ctx, url)
	if err != nil {
		return nil, fmt.Errorf("download failed: %w", err)
	}

	// 2. Submeter a tarefa de conversão ao worker pool
	var task TaskWithContext
	switch mediaType {
	case "image":
		task = func(ctx context.Context) ([]byte, error) {
			return s.convertToWebp(ctx, originalData)
		}
	case "audio":
		task = func(ctx context.Context) ([]byte, error) {
			// A função de áudio agora retorna mais dados, mas para compatibilidade
			// com a interface genérica, descartamos os extras aqui.
			converted, _, _, err := s.convertToOpus(ctx, originalData)
			return converted, err
		}
	default:
		// Para outros tipos como vídeo e documento, retornamos o original.
		return originalData, nil
	}

	resultChan, err := s.workerPool.SubmitWithContext(ctx, task)
	if err != nil {
		return nil, fmt.Errorf("failed to submit task to worker pool: %w", err)
	}

	select {
	case <-ctx.Done():
		return nil, ctx.Err()
	case result := <-resultChan:
		return result.data, result.err
	}
}

// ProcessAudio é uma função especializada para áudio que retorna metadados adicionais.
func (s *ConverterService) ProcessAudio(ctx context.Context, url string) ([]byte, []byte, float64, error) {
	originalData, err := s.download(ctx, url)
	if err != nil {
		return nil, nil, 0, fmt.Errorf("download failed: %w", err)
	}

	return s.convertToOpus(ctx, originalData)
}

func (s *ConverterService) download(ctx context.Context, url string) ([]byte, error) {
	req, err := http.NewRequestWithContext(ctx, "GET", url, nil)
	if err != nil {
		return nil, err
	}
	req.Header.Set("User-Agent", "WhatsMiau-Converter/2.0")

	resp, err := s.httpClient.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("http status %d", resp.StatusCode)
	}

	buf := GetBuffer()
	defer PutBuffer(buf)

	_, err = io.Copy(buf, resp.Body)
	if err != nil {
		return nil, err
	}
	return buf.Bytes(), nil
}

func (s *ConverterService) convertToWebp(ctx context.Context, input []byte) ([]byte, error) {
	cmd := exec.CommandContext(ctx, "vips",
		"webpsave_buffer",
		"--preset=default",
		"--Q=80",
		"--strip", // Remove metadata
		"-",       // Input from stdin
		"-",       // Output to stdout
	)
	cmd.Stdin = bytes.NewReader(input)
	var outBuffer, errBuffer bytes.Buffer
	cmd.Stdout = &outBuffer
	cmd.Stderr = &errBuffer

	if err := cmd.Run(); err != nil {
		// Fallback para ffmpeg se vips falhar
		zap.L().Warn("vips conversion failed, falling back to ffmpeg", zap.Error(err), zap.String("stderr", errBuffer.String()))
		return s.convertToWebpFFmpeg(ctx, input)
	}
	return outBuffer.Bytes(), nil
}

func (s *ConverterService) convertToWebpFFmpeg(ctx context.Context, input []byte) ([]byte, error) {
	cmd := exec.CommandContext(ctx, "ffmpeg",
		"-hide_banner",
		"-loglevel", "error",
		"-i", "pipe:0",
		"-vf", "scale='min(1280,iw)':'min(1280,ih)':force_original_aspect_ratio=decrease",
		"-q:v", "80",
		"-f", "webp",
		"pipe:1",
	)
	cmd.Stdin = bytes.NewReader(input)
	var outBuffer, errBuffer bytes.Buffer
	cmd.Stdout = &outBuffer
	cmd.Stderr = &errBuffer

	if err := cmd.Run(); err != nil {
		return nil, fmt.Errorf("ffmpeg fallback error: %v, stderr: %s", err, errBuffer.String())
	}
	return outBuffer.Bytes(), nil
}

// FIX: A função foi completamente reescrita para extrair duração e gerar a waveform.
func (s *ConverterService) convertToOpus(ctx context.Context, input []byte) (opusData []byte, waveform []byte, duration float64, err error) {
	// Etapa 1: Extrair a duração do áudio com ffprobe
	durationCmd := exec.CommandContext(ctx, "ffprobe",
		"-i", "pipe:0",
		"-show_entries", "format=duration",
		"-v", "quiet",
		"-of", "csv=p=0",
	)
	durationCmd.Stdin = bytes.NewReader(input)
	var durationOut bytes.Buffer
	durationCmd.Stdout = &durationOut
	if err = durationCmd.Run(); err != nil {
		return nil, nil, 0, fmt.Errorf("ffprobe failed to get duration: %w", err)
	}
	duration, _ = strconv.ParseFloat(strings.TrimSpace(durationOut.String()), 64)

	// Etapa 2: Converter o áudio principal para Opus
	opusCmd := exec.CommandContext(ctx, "ffmpeg",
		"-hide_banner", "-loglevel", "error",
		"-i", "pipe:0",
		"-c:a", "libopus", "-b:a", "128k", "-vbr", "on",
		"-compression_level", "10", "-application", "voip",
		"-ar", "48000", "-ac", "1",
		"-f", "opus", "pipe:1",
	)
	opusCmd.Stdin = bytes.NewReader(input)
	var opusBuffer, opusErrBuffer bytes.Buffer
	opusCmd.Stdout = &opusBuffer
	opusCmd.Stderr = &opusErrBuffer
	if err = opusCmd.Run(); err != nil {
		return nil, nil, 0, fmt.Errorf("ffmpeg opus conversion error: %v, stderr: %s", err, opusErrBuffer.String())
	}
	opusData = opusBuffer.Bytes()

	// Etapa 3: Gerar dados brutos para a waveform
	waveformCmd := exec.CommandContext(ctx, "ffmpeg",
		"-hide_banner", "-loglevel", "error",
		"-i", "pipe:0",
		"-f", "s8", // Formato de 8-bit assinado
		"-ar", "8000", // Baixa taxa de amostragem
		"-ac", "1", // Mono
		"pipe:1",
	)
	waveformCmd.Stdin = bytes.NewReader(input)
	var waveformBuffer, waveformErrBuffer bytes.Buffer
	waveformCmd.Stdout = &waveformBuffer
	waveformCmd.Stderr = &waveformErrBuffer
	if err = waveformCmd.Run(); err != nil {
		return nil, nil, 0, fmt.Errorf("ffmpeg waveform generation error: %v, stderr: %s", err, waveformErrBuffer.String())
	}

	// Etapa 4: Amostrar os dados brutos para criar a waveform final (64 barras)
	rawData := waveformBuffer.Bytes()
	totalSamples := len(rawData)
	samplesPerBar := totalSamples / 64
	if samplesPerBar == 0 {
		samplesPerBar = 1
	}

	finalWaveform := make([]byte, 64)
	for i := 0; i < 64; i++ {
		start := i * samplesPerBar
		end := start + samplesPerBar
		if end > totalSamples {
			end = totalSamples
		}
		if start >= end {
			if i > 0 {
				finalWaveform[i] = finalWaveform[i-1] // Evita barras vazias no final
			}
			continue
		}

		// Encontra o valor de pico (amplitude máxima) no segmento
		var peak byte
		for _, sample := range rawData[start:end] {
			val := sample
			if val > 128 { // Normaliza valores negativos
				val = 255 - val
			}
			if val > peak {
				peak = val
			}
		}
		// Limita o valor máximo para a visualização
		if peak > 100 {
			peak = 100
		}
		finalWaveform[i] = peak
	}
	waveform = finalWaveform

	return
}
