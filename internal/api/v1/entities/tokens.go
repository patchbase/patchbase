package entities

import (
	"go.patchbase.net/server/internal/services"
	"go.patchbase.net/server/internal/utils"
)

type RegistrationToken struct {
	ID              string             `json:"id"`
	Name            string             `json:"name"`
	CreatedByUserID string             `json:"created_by_user_id"`
	CreatedAt       string             `json:"created_at"`
	RevokedAt       utils.Option[string] `json:"revoked_at"`
	LastUsedAt      utils.Option[string] `json:"last_used_at"`
}

type CreatedRegistrationToken struct {
	ID        string `json:"id"`
	Name      string `json:"name"`
	Token     string `json:"token"`
	CreatedAt string `json:"created_at"`
}

func NewRegistrationToken(value services.RegistrationTokenInfo) RegistrationToken {
	return RegistrationToken{
		ID:              value.ID,
		Name:            value.Name,
		CreatedByUserID: value.CreatedBy,
		CreatedAt:       value.CreatedAt.UTC().Format(apiTimeLayout),
		RevokedAt:       NewOptionalFormattedTime(value.RevokedAt),
		LastUsedAt:      NewOptionalFormattedTime(value.LastUsedAt),
	}
}

func NewRegistrationTokens(values []services.RegistrationTokenInfo) []RegistrationToken {
	result := make([]RegistrationToken, 0, len(values))
	for _, value := range values {
		result = append(result, NewRegistrationToken(value))
	}
	return result
}

func NewCreatedRegistrationToken(value services.CreatedRegistrationToken) CreatedRegistrationToken {
	return CreatedRegistrationToken{
		ID:        value.ID,
		Name:      value.Name,
		Token:     value.Token,
		CreatedAt: value.CreatedAt.UTC().Format(apiTimeLayout),
	}
}
