package outlookweb

import (
	"errors"
	"net/http"
	"net/http/httptest"
	"sync/atomic"
	"testing"

	"github.com/nkiyohara/corresync/internal/application"
)

func TestSavedDraftSendClassifiesFailuresWithoutRetry(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name        string
		status      int
		response    string
		wantUnknown bool
	}{
		{
			name: "stale version", status: http.StatusOK,
			response: `{"Body":{"ResponseMessages":{"Items":[{"ResponseClass":"Error","ResponseCode":"ErrorStaleObject"}]}}}`,
		},
		{
			name: "conflict", status: http.StatusOK,
			response: `{"Body":{"ResponseMessages":{"Items":[{"ResponseClass":"Error","ResponseCode":"ErrorIrresolvableConflict"}]}}}`,
		},
		{
			name: "send denied", status: http.StatusOK,
			response: `{"Body":{"ResponseMessages":{"Items":[{"ResponseClass":"Error","ResponseCode":"ErrorSendAsDenied"}]}}}`,
		},
		{
			name: "server error", status: http.StatusInternalServerError,
			response: `{}`, wantUnknown: true,
		},
		{
			name: "warning", status: http.StatusOK,
			response:    `{"Body":{"ResponseMessages":{"Items":[{"ResponseClass":"Warning","ResponseCode":"ErrorBatchProcessingStopped"}]}}}`,
			wantUnknown: true,
		},
		{
			name: "missing response", status: http.StatusOK,
			response: `{"Body":{"ResponseMessages":{"Items":[]}}}`, wantUnknown: true,
		},
		{
			name: "multiple responses", status: http.StatusOK,
			response:    `{"Body":{"ResponseMessages":{"Items":[{"ResponseClass":"Success","ResponseCode":"NoError"},{"ResponseClass":"Success","ResponseCode":"NoError"}]}}}`,
			wantUnknown: true,
		},
		{
			name: "malformed response", status: http.StatusOK,
			response: `{`, wantUnknown: true,
		},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			t.Parallel()
			var calls atomic.Int32
			server := httptest.NewTLSServer(http.HandlerFunc(func(writer http.ResponseWriter, request *http.Request) {
				calls.Add(1)
				if request.URL.Query().Get("action") != "UpdateItem" {
					t.Errorf("unexpected action %q", request.URL.Query().Get("action"))
				}
				writer.WriteHeader(test.status)
				_, _ = writer.Write([]byte(test.response))
			}))
			defer server.Close()
			_, err := testClient(t, server, nil).SendMailDraft(t.Context(), application.MailDraftSendInput{
				Account: "work", DraftID: "draft-1", DraftChangeKey: "change-1",
			})
			if err == nil || errors.Is(err, application.ErrWriteOutcomeUnknown) != test.wantUnknown {
				t.Fatalf("SendMailDraft() error = %v, wantUnknown %t", err, test.wantUnknown)
			}
			if calls.Load() != 1 {
				t.Fatalf("send calls = %d, want exactly 1", calls.Load())
			}
		})
	}
}
