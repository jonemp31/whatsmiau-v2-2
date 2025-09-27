package converter

import (
	"bytes"
	"context"
	"fmt"
	"io"
	"net/http"
	"os/exec"
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

func (s *ConverterService) convertToOpus(ctx context.Context, input []byte) ([]byte, []byte, float64, error) {
	cmd := exec.CommandContext(ctx, "ffmpeg",
		"-hide_banner",
		"-loglevel", "error",

		"-i", "pipe:0",
		"-c:a", "libopus",
		"-b:a", "128k",
		"-vbr", "on",
		"-compression_level", "10",
		"-application", "voip",
		"-ar", "48000",
		"-ac", "1",
		"-f", "opus",
		"pipe:1",
	)
	cmd.Stdin = bytes.NewReader(input)
	var outBuffer, errBuffer bytes.Buffer
	cmd.Stdout = &outBuffer
	cmd.Stderr = &errBuffer

	if err := cmd.Run(); err != nil {
		return nil, nil, 0, fmt.Errorf("ffmpeg error: %v, stderr: %s", err, errBuffer.String())
	}

	// Para manter a compatibilidade com a assinatura da função antiga,
	// retornamos waveform e duration vazios/zero por enquanto.
	// A lógica de extração pode ser adicionada aqui se necessário.
	return outBuffer.Bytes(), []byte{}, 0, nil
}
