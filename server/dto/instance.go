package dto

import "github.com/verbeux-ai/whatsmiau/models"

type CreateInstanceRequest struct {
	ID               string `json:"id,omitempty" validate:"required_without=InstanceName"`
	InstanceName     string `json:"instanceName,omitempty" validate:"required_without=InstanceID"`
	*models.Instance        // optional arguments
}

type CreateInstanceResponse struct {
	*models.Instance
}

type UpdateInstanceRequest struct {
	ID      string `json:"id,omitempty" param:"id" validate:"required"`
	Webhook struct {
		Base64 bool `json:"base64,omitempty"`
	} `json:"webhook,omitempty"`
}

type UpdateInstanceResponse struct {
	*models.Instance
}

type ListInstancesRequest struct {
	InstanceName string `query:"instanceName"`
	ID           string `query:"id"`
}

type ListInstancesResponse struct {
	*models.Instance

	OwnerJID string `json:"ownerJid,omitempty"`
	Status   string `json:"status,omitempty"`
}

type ConnectInstanceRequest struct {
	ID string `param:"id" validate:"required"`
}

type ConnectInstanceResponse struct {
	Message   string `json:"message,omitempty"`
	Connected bool   `json:"connected,omitempty"`
	Base64    string `json:"base64,omitempty"`
	*models.Instance
}

type StatusInstanceRequest struct {
	ID string `param:"id" validate:"required"`
}

type StatusInstanceResponse struct {
	ID        string                                        `json:"id,omitempty"`
	Status    string                                        `json:"state,omitempty"`
	RemoteJID string                                        `json:"remoteJid,omitempty"`
	Instance  *StatusInstanceResponseEvolutionCompatibility `json:"instance,omitempty"`
}

type StatusInstanceResponseEvolutionCompatibility struct {
	InstanceName string `json:"instanceName,omitempty"`
	State        string `json:"state,omitempty"`
}

type DeleteInstanceRequest struct {
	ID string `param:"id" validate:"required"`
}

type DeleteInstanceResponse struct {
	Message string `json:"message,omitempty"`
}

type LogoutInstanceRequest struct {
	ID string `param:"id" validate:"required"`
}

type LogoutInstanceResponse struct {
	Message string `json:"message,omitempty"`
}

// Pairing Code DTOs
type StartPairingRequest struct {
	ID          string `param:"id" validate:"required"`
	PhoneNumber string `json:"phoneNumber" validate:"required"`
	ClientType  string `json:"clientType,omitempty"` // "chrome", "firefox", "safari", "edge"
	ClientName  string `json:"clientName,omitempty"` // "Chrome (Windows)", "Safari (iOS)"
}

type StartPairingResponse struct {
	Success     bool   `json:"success"`
	PairingCode string `json:"pairingCode,omitempty"` // XXXX-XXXX
	Message     string `json:"message"`
	SessionID   string `json:"sessionId,omitempty"`
	ExpiresAt   int64  `json:"expiresAt,omitempty"`
}

type PairingStatusRequest struct {
	ID        string `param:"id" validate:"required"`
	SessionID string `query:"sessionId" validate:"required"`
}

type PairingStatusResponse struct {
	Status    string `json:"status"` // "pending", "success", "failed", "expired"
	Message   string `json:"message"`
	SessionID string `json:"sessionId,omitempty"`
	ExpiresAt int64  `json:"expiresAt,omitempty"`
}

type UpdateReadSettingsRequest struct {
	ID               string `json:"id" param:"id" validate:"required"`
	AutoReadMessages bool   `json:"autoReadMessages"`
	ReadDelay        int    `json:"readDelay" validate:"min=0,max=300"` // 0-5 minutos
}

type UpdateReadSettingsResponse struct {
	Instance *models.Instance `json:"instance"`
}

type PairingCodeRequest struct {
	PhoneNumber string `json:"phoneNumber" validate:"required"`
}

type PairingCodeResponse struct {
	Code string `json:"code"`
}

type UpdateWebhookRequest struct {
	Url      string   `json:"url"`
	ByEvents bool     `json:"byEvents"`
	Events   []string `json:"events"`
}
