package main

import (
	"bytes"
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"runtime"
	"strings"
	"testing"
)

func TestMEDIA001M2CDaemonRestartRoundTrip(t *testing.T) {
	if runtime.GOOS != "linux" {
		t.Skip("authenticated local media access is Linux-only")
	}
	linked := startLinkedPaymentDaemon(t)
	first := waitLocalClientDescriptor(t, linked.dataDir, localClientReadyWait)
	id := strings.Repeat("6a", 16)
	content := bytes.Repeat([]byte("daemon-media"), 10000)

	reference := m2cPut(t, first, id, content)
	firstHandle := m2cCreateHandle(t, first, id)
	if got := m2cDownload(t, first, firstHandle); !bytes.Equal(got, content) {
		t.Fatalf("first download bytes = %d, want %d", len(got), len(content))
	}
	assertMediaSecretsAbsentFromLogs(t, linked.child, first.Token, firstHandle)

	linked.stop(t)
	linked.startChild(t)
	second := waitLocalClientDescriptor(t, linked.dataDir, localClientReadyWait)
	if second.Token == first.Token || second.InstanceID == first.InstanceID {
		t.Fatal("restart reused local-client credentials")
	}

	res := m2cRequest(t, second.Endpoint, first.Token, first.InstanceID, http.MethodGet, "/v1/media/public/"+id, nil)
	if res.StatusCode != http.StatusUnauthorized {
		body, _ := io.ReadAll(res.Body)
		res.Body.Close()
		t.Fatalf("old credential status=%d body=%s", res.StatusCode, body)
	}
	res.Body.Close()
	res = m2cRequest(t, second.Endpoint, second.Token, first.InstanceID, http.MethodGet, "/v1/media/public/"+id, nil)
	if res.StatusCode != http.StatusUnauthorized {
		body, _ := io.ReadAll(res.Body)
		res.Body.Close()
		t.Fatalf("stale instance status=%d body=%s", res.StatusCode, body)
	}
	res.Body.Close()

	res = m2cRequest(t, second.Endpoint, second.Token, second.InstanceID, http.MethodGet, "/v1/media/handles/"+firstHandle, nil)
	if res.StatusCode != http.StatusNotFound {
		body, _ := io.ReadAll(res.Body)
		res.Body.Close()
		t.Fatalf("old handle status=%d body=%s", res.StatusCode, body)
	}
	res.Body.Close()

	reopened := m2cGetReference(t, second, id)
	if reopened.CID != reference.CID || reopened.ByteLength != reference.ByteLength {
		t.Fatalf("reopened reference=%+v want=%+v", reopened, reference)
	}
	secondHandle := m2cCreateHandle(t, second, id)
	if secondHandle == firstHandle {
		t.Fatal("restart reused a download handle")
	}
	if got := m2cDownload(t, second, secondHandle); !bytes.Equal(got, content) {
		t.Fatalf("reopened download bytes = %d, want %d", len(got), len(content))
	}
	if records := getPaymentRecords(t, second); len(records) != 0 {
		t.Fatalf("payment route records = %d, want 0", len(records))
	}

	res = m2cRequest(t, second.Endpoint, second.Token, second.InstanceID, http.MethodDelete, "/v1/media/public/"+id, nil)
	if body, err := io.ReadAll(res.Body); err != nil || res.StatusCode != http.StatusNoContent || len(body) != 0 {
		res.Body.Close()
		t.Fatalf("release status=%d body=%q err=%v", res.StatusCode, body, err)
	}
	res.Body.Close()
	res = m2cRequest(t, second.Endpoint, second.Token, second.InstanceID, http.MethodGet, "/v1/media/public/"+id, nil)
	if res.StatusCode != http.StatusNotFound {
		body, _ := io.ReadAll(res.Body)
		res.Body.Close()
		t.Fatalf("released reference status=%d body=%s", res.StatusCode, body)
	}
	res.Body.Close()
	assertMediaSecretsAbsentFromLogs(t, linked.child, first.Token, second.Token, firstHandle, secondHandle)
}

type m2cReference struct {
	V           int    `json:"v"`
	PeerID      string `json:"peer_id"`
	InstanceID  string `json:"instance_id"`
	ReferenceID string `json:"reference_id"`
	CID         string `json:"cid"`
	ByteLength  int64  `json:"byte_length"`
}

func m2cPut(t *testing.T, desc localClientDescriptor, id string, content []byte) m2cReference {
	t.Helper()
	res := m2cRequest(t, desc.Endpoint, desc.Token, desc.InstanceID, http.MethodPut, "/v1/media/public/"+id, bytes.NewReader(content))
	defer res.Body.Close()
	if res.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(res.Body)
		t.Fatalf("upload status=%d body=%s", res.StatusCode, body)
	}
	return decodeM2CReference(t, res.Body)
}

func m2cGetReference(t *testing.T, desc localClientDescriptor, id string) m2cReference {
	t.Helper()
	res := m2cRequest(t, desc.Endpoint, desc.Token, desc.InstanceID, http.MethodGet, "/v1/media/public/"+id, nil)
	defer res.Body.Close()
	if res.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(res.Body)
		t.Fatalf("reference status=%d body=%s", res.StatusCode, body)
	}
	return decodeM2CReference(t, res.Body)
}

func m2cCreateHandle(t *testing.T, desc localClientDescriptor, id string) string {
	t.Helper()
	res := m2cRequest(t, desc.Endpoint, desc.Token, desc.InstanceID, http.MethodPost, "/v1/media/public/"+id+"/handles", nil)
	defer res.Body.Close()
	if res.StatusCode != http.StatusCreated {
		body, _ := io.ReadAll(res.Body)
		t.Fatalf("handle status=%d body=%s", res.StatusCode, body)
	}
	var payload struct {
		Handle string `json:"handle"`
	}
	if err := json.NewDecoder(res.Body).Decode(&payload); err != nil {
		t.Fatal(err)
	}
	if len(payload.Handle) != 32 {
		t.Fatalf("handle = %q", payload.Handle)
	}
	return payload.Handle
}

func m2cDownload(t *testing.T, desc localClientDescriptor, handle string) []byte {
	t.Helper()
	res := m2cRequest(t, desc.Endpoint, desc.Token, desc.InstanceID, http.MethodGet, "/v1/media/handles/"+handle, nil)
	defer res.Body.Close()
	body, err := io.ReadAll(res.Body)
	if err != nil {
		t.Fatal(err)
	}
	if res.StatusCode != http.StatusOK || res.Header.Get("Content-Type") != "application/octet-stream" || res.Header.Get("Content-Disposition") != "attachment" {
		t.Fatalf("download status=%d headers=%v body=%s", res.StatusCode, res.Header, body[:min(len(body), 64)])
	}
	return body
}

func m2cRequest(t *testing.T, endpoint, token, instance, method, path string, body io.Reader) *http.Response {
	t.Helper()
	req, err := http.NewRequest(method, endpoint+path, body)
	if err != nil {
		t.Fatal(err)
	}
	req.Header.Set("Authorization", "Bearer "+token)
	req.Header.Set("X-BitBook-Instance", instance)
	if method == http.MethodPut {
		req.Header.Set("Content-Type", "application/octet-stream")
	}
	client := &http.Client{
		Timeout:       daemonOpTimeout,
		CheckRedirect: func(*http.Request, []*http.Request) error { return http.ErrUseLastResponse },
		Transport:     &http.Transport{Proxy: nil, DisableKeepAlives: true},
	}
	res, err := client.Do(req)
	if err != nil {
		t.Fatal(err)
	}
	return res
}

func decodeM2CReference(t *testing.T, reader io.Reader) m2cReference {
	t.Helper()
	decoder := json.NewDecoder(reader)
	decoder.DisallowUnknownFields()
	var result m2cReference
	if err := decoder.Decode(&result); err != nil {
		t.Fatal(err)
	}
	if result.V != 1 || result.ReferenceID == "" || result.CID == "" || result.ByteLength < 0 {
		t.Fatalf("reference response = %+v", result)
	}
	if err := decoder.Decode(&struct{}{}); !errors.Is(err, io.EOF) {
		t.Fatalf("reference response trailing JSON: %v", err)
	}
	return result
}

func assertMediaSecretsAbsentFromLogs(t *testing.T, child *paymentDaemonChild, values ...string) {
	t.Helper()
	logs := child.stdout.String() + child.stderr.String()
	for _, value := range values {
		if value != "" && strings.Contains(logs, value) {
			t.Fatal("captured daemon logs contain media handle material")
		}
	}
}
