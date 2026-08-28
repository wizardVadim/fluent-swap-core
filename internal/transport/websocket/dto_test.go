package websocket

import (
	"encoding/json"
	"reflect"
	"testing"
)

func TestDTOJSONContract(t *testing.T) {
	tests := []struct {
		name    string
		message any
		want    map[string]any
	}{
		{
			name: "find partner contains language payload",
			message: FindPartner{
				Type:      TypeFindPartner,
				RequestID: "req-1234",
				Payload: FindPartnerPayload{
					NativeLanguageCode:   "ru",
					LearningLanguageCode: "en",
				},
			},
			want: map[string]any{
				"type":       "find_partner",
				"request_id": "req-1234",
				"payload": map[string]any{
					"native_language_code":   "ru",
					"learning_language_code": "en",
				},
			},
		},
		{
			name: "cancel search omits payload",
			message: CancelSearch{
				Type:      TypeCancelSearch,
				RequestID: "req-1235",
			},
			want: map[string]any{
				"type":       "cancel_search",
				"request_id": "req-1235",
			},
		},
		{
			name: "error omits absent request id",
			message: Error{
				Type: TypeError,
				Payload: ErrorPayload{
					Code:    ErrorInvalidJSON,
					Message: "message is not valid JSON",
				},
			},
			want: map[string]any{
				"type": "error",
				"payload": map[string]any{
					"code":    "invalid_json",
					"message": "message is not valid JSON",
				},
			},
		},
		{
			name: "match found contains match id",
			message: MatchFound{
				Type:      TypeMatchFound,
				RequestID: "req-1234",
				Payload: MatchFoundPayload{
					MatchID: "match-5678",
				},
			},
			want: map[string]any{
				"type":       "match_found",
				"request_id": "req-1234",
				"payload": map[string]any{
					"match_id": "match-5678",
				},
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			encoded, err := json.Marshal(tt.message)
			if err != nil {
				t.Fatalf("marshal message: %v", err)
			}

			var got map[string]any
			if err := json.Unmarshal(encoded, &got); err != nil {
				t.Fatalf("unmarshal encoded message: %v", err)
			}

			if !reflect.DeepEqual(got, tt.want) {
				t.Errorf("JSON contract mismatch: got %v, want %v", got, tt.want)
			}
		})
	}
}
