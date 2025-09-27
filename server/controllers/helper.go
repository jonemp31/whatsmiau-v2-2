package controllers

import (
	"fmt"
	"regexp"
	"strings"
	"unicode"

	"go.mau.fi/whatsmeow/types"
)

// numberToJid converte um número de telefone para JID do WhatsApp com validação robusta
func numberToJid(number string) (*types.JID, error) {
	// 1. Sanitização básica
	number = strings.TrimSpace(number)
	if number == "" {
		return nil, fmt.Errorf("phone number cannot be empty")
	}

	// 2. Remover caracteres de formatação comuns
	number = strings.ReplaceAll(number, " ", "")
	number = strings.ReplaceAll(number, "-", "")
	number = strings.ReplaceAll(number, "(", "")
	number = strings.ReplaceAll(number, ")", "")
	number = strings.ReplaceAll(number, ".", "")

	// 3. Verificar se contém apenas caracteres válidos (números e +)
	if !isValidPhoneNumber(number) {
		return nil, fmt.Errorf("phone number contains invalid characters")
	}

	// 4. Validar formato usando regex
	if !regexp.MustCompile(`^\+?[1-9]\d{10,14}$`).MatchString(number) {
		return nil, fmt.Errorf("invalid phone number format - must be 11-15 digits with optional country prefix")
	}

	// 5. Adicionar sufixo do WhatsApp se necessário
	if !strings.Contains(number, "@") {
		number += "@s.whatsapp.net"
	}

	// 6. Parse final
	jid, err := types.ParseJID(number)
	if err != nil {
		return nil, fmt.Errorf("failed to parse JID: %w", err)
	}

	return &jid, nil
}

// isValidPhoneNumber verifica se o número contém apenas caracteres válidos
func isValidPhoneNumber(number string) bool {
	for _, char := range number {
		if !unicode.IsDigit(char) && char != '+' {
			return false
		}
	}
	return true
}
