package websocket

type Type string

const (
	TypeFindPartner     Type = "find_partner"
	TypeCancelSearch    Type = "cancel_search"
	TypeSearchWaiting   Type = "search_waiting"
	TypeSearchCancelled Type = "search_cancelled"
	TypeMatchFound      Type = "match_found"
	TypeError           Type = "error"
)

// client -> server
type FindPartner struct {
	Type      Type               `json:"type"`
	RequestID string             `json:"request_id"`
	Payload   FindPartnerPayload `json:"payload"`
}

type FindPartnerPayload struct {
	NativeLanguageCode   string `json:"native_language_code"`
	LearningLanguageCode string `json:"learning_language_code"`
}

// client -> server
type CancelSearch struct {
	Type      Type   `json:"type"`
	RequestID string `json:"request_id"`
}

// server -> client
type SearchWaiting struct {
	Type      Type   `json:"type"`
	RequestID string `json:"request_id"`
}

// server -> client
type SearchCancelled struct {
	Type      Type   `json:"type"`
	RequestID string `json:"request_id"`
}

// server -> client
type MatchFound struct {
	Type      Type              `json:"type"`
	RequestID string            `json:"request_id"`
	Payload   MatchFoundPayload `json:"payload"`
}

type MatchFoundPayload struct {
	MatchID string `json:"match_id"`
}

// server -> client
type Error struct {
	Type      Type         `json:"type"`
	RequestID *string      `json:"request_id,omitempty"`
	Payload   ErrorPayload `json:"payload"`
}

type ErrorPayload struct {
	Code    ErrorCode `json:"code"`
	Message string    `json:"message"`
}

type ErrorCode string

const (
	ErrorInvalidJSON         ErrorCode = "invalid_json"
	ErrorUnknownMessageType  ErrorCode = "unknown_message_type"
	ErrorInvalidPayload      ErrorCode = "invalid_payload"
	ErrorInternalServerError ErrorCode = "internal_server_error"
)
