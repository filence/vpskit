package publisher

import (
	"crypto/hmac"
	"crypto/sha256"
	"encoding/base64"
	"encoding/hex"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"vpskit.local/vpskit/internal/artifact"
)

func TestWorkersPublishAndReadback(t *testing.T) {
	secret := strings.Repeat("p", 43)
	readToken := strings.Repeat("r", 43)
	now := time.Date(2026, 7, 21, 4, 0, 0, 0, time.UTC)
	stored := map[string][]byte{}
	server := httptest.NewServer(http.HandlerFunc(func(response http.ResponseWriter, request *http.Request) {
		switch {
		case request.URL.Path == "/healthz":
			response.WriteHeader(http.StatusOK)
		case request.Method == http.MethodPost && strings.HasSuffix(request.URL.Path, "/publish"):
			body, _ := io.ReadAll(request.Body)
			if !validTestSignature(request, body, secret) {
				response.WriteHeader(http.StatusUnauthorized)
				return
			}
			var payload PublicationPayload
			if err := json.Unmarshal(body, &payload); err != nil {
				t.Fatal(err)
			}
			for _, item := range payload.Artifacts {
				decoded, err := decodeBase64(item.ContentBase64)
				if err != nil {
					t.Fatal(err)
				}
				stored[item.Target] = decoded
			}
			_ = json.NewEncoder(response).Encode(PublicationResult{Status: "COMMITTED", PublicationID: payload.PublicationID, NodeRevision: payload.NodeRevision, PublishedAt: now, Targets: []string{"mihomo", "v2rayn", "manifest"}})
		case request.Method == http.MethodGet && strings.HasPrefix(request.URL.Path, "/s/"+readToken+"/"):
			target := strings.TrimPrefix(request.URL.Path, "/s/"+readToken+"/")
			content, exists := stored[target]
			if !exists {
				response.WriteHeader(http.StatusNotFound)
				return
			}
			_, _ = response.Write(content)
		default:
			response.WriteHeader(http.StatusNotFound)
		}
	}))
	defer server.Close()

	set := testWorkersArtifactSet(t)
	publisher, err := NewWorkers(WorkersConfig{Endpoint: server.URL, NodeID: "node-main", PublishSecret: secret, ReadToken: readToken, HTTPClient: server.Client(), Now: func() time.Time { return now }})
	if err != nil {
		t.Fatal(err)
	}
	result, err := publisher.PublishWithResult(t.Context(), set)
	if err != nil {
		t.Fatal(err)
	}
	if result.Status != "COMMITTED" || len(stored) != 3 {
		t.Fatalf("unexpected publication result: %#v stored=%d", result, len(stored))
	}
	if err := publisher.Healthcheck(); err != nil {
		t.Fatal(err)
	}
}

func TestWorkersRejectsMismatchedNodeAndDoesNotLeakSecret(t *testing.T) {
	secret := strings.Repeat("s", 43)
	publisher, err := NewWorkers(WorkersConfig{Endpoint: "http://127.0.0.1", NodeID: "expected", PublishSecret: secret, ReadToken: strings.Repeat("r", 43)})
	if err != nil {
		t.Fatal(err)
	}
	set := testWorkersArtifactSet(t)
	if err := publisher.Preflight(set); err == nil || strings.Contains(err.Error(), secret) {
		t.Fatalf("expected sanitized node mismatch, got %v", err)
	}
}

func TestWorkersTokenRotationValidatesEntropyAndOverlap(t *testing.T) {
	publisher, err := NewWorkers(WorkersConfig{Endpoint: "http://127.0.0.1", NodeID: "node-main", PublishSecret: strings.Repeat("s", 43), ReadToken: strings.Repeat("r", 43)})
	if err != nil {
		t.Fatal(err)
	}
	if _, err := publisher.RotateReadTokenTo(t.Context(), "short", time.Hour); err == nil {
		t.Fatal("short token must be rejected")
	}
	if _, err := publisher.RotateReadTokenTo(t.Context(), strings.Repeat("n", 43), 8*24*time.Hour); err == nil {
		t.Fatal("excessive overlap must be rejected")
	}
}

func TestWorkersReadbackWaitsForACompleteRevision(t *testing.T) {
	secret := strings.Repeat("s", 43)
	readToken := strings.Repeat("r", 43)
	set := testWorkersArtifactSet(t)
	payload, err := publicationPayload(set)
	if err != nil {
		t.Fatal(err)
	}
	contents := map[string][]byte{}
	for _, item := range payload.Artifacts {
		contents[item.Target], err = decodeBase64(item.ContentBase64)
		if err != nil {
			t.Fatal(err)
		}
	}
	requests := 0
	server := httptest.NewServer(http.HandlerFunc(func(response http.ResponseWriter, request *http.Request) {
		requests++
		target := strings.TrimPrefix(request.URL.Path, "/s/"+readToken+"/")
		if requests == 1 {
			_, _ = response.Write([]byte("stale\n"))
			return
		}
		content, ok := contents[target]
		if !ok {
			response.WriteHeader(http.StatusNotFound)
			return
		}
		_, _ = response.Write(content)
	}))
	defer server.Close()
	publisher, err := NewWorkers(WorkersConfig{
		Endpoint: server.URL, NodeID: "node-main", PublishSecret: secret, ReadToken: readToken,
		HTTPClient: server.Client(), ReadbackWait: 250 * time.Millisecond, ReadbackPoll: time.Millisecond,
	})
	if err != nil {
		t.Fatal(err)
	}
	if err := publisher.ReadbackContext(t.Context(), set); err != nil {
		t.Fatal(err)
	}
	if requests < 4 {
		t.Fatalf("readback did not retry a stale response: requests=%d", requests)
	}
}

func testWorkersArtifactSet(t *testing.T) artifact.Set {
	t.Helper()
	capability := artifact.Capability{Name: "test", RendererVersion: 1, CompatibilityProfile: "test/v1"}
	mihomo, _ := artifact.New("mihomo.yaml", "mihomo", "text/yaml", true, capability, []byte("proxies: []\n"))
	links, _ := artifact.New("share-links.txt", "share-links", "text/plain", true, capability, []byte("vless://example\n"))
	set, err := artifact.NewSet("node-main", 7, 0, []artifact.Artifact{mihomo, links})
	if err != nil {
		t.Fatal(err)
	}
	return set
}

func validTestSignature(request *http.Request, body []byte, secret string) bool {
	digest := sha256.Sum256(body)
	canonical := strings.Join([]string{request.Header.Get("X-VPSKit-Timestamp"), request.Header.Get("X-VPSKit-Nonce"), request.Method, request.URL.Path, hex.EncodeToString(digest[:])}, "\n")
	mac := hmac.New(sha256.New, []byte(secret))
	_, _ = mac.Write([]byte(canonical))
	expected := hex.EncodeToString(mac.Sum(nil))
	return hmac.Equal([]byte(expected), []byte(request.Header.Get("X-VPSKit-Signature")))
}

func decodeBase64(value string) ([]byte, error) {
	return base64.StdEncoding.DecodeString(value)
}
