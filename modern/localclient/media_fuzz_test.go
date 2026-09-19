package localclient

import (
	"bytes"
	"net"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strconv"
	"strings"
	"testing"
)

const (
	fuzzDuplicate    = "\x00duplicate"
	fuzzEmptyPresent = "\x00empty-present"
)

func TestMEDIA001M2CFuzzAuthFixture(t *testing.T) {
	tests := []struct {
		name          string
		authorization string
		instance      string
		origin        string
		wantStatus    int
		wantAuth      int
		wantInstance  int
		wantOrigin    int
	}{
		{name: "valid", authorization: "Bearer token", instance: "instance", wantStatus: 0, wantAuth: 1, wantInstance: 1},
		{name: "missing bearer", instance: "instance", wantStatus: http.StatusUnauthorized, wantInstance: 1},
		{name: "empty bearer", authorization: fuzzEmptyPresent, instance: "instance", wantStatus: http.StatusUnauthorized, wantAuth: 1, wantInstance: 1},
		{name: "duplicate bearer", authorization: fuzzDuplicate, instance: "instance", wantStatus: http.StatusUnauthorized, wantAuth: 2, wantInstance: 1},
		{name: "missing instance", authorization: "Bearer token", wantStatus: http.StatusUnauthorized, wantAuth: 1},
		{name: "empty instance", authorization: "Bearer token", instance: fuzzEmptyPresent, wantStatus: http.StatusUnauthorized, wantAuth: 1, wantInstance: 1},
		{name: "duplicate instance", authorization: "Bearer token", instance: fuzzDuplicate, wantStatus: http.StatusUnauthorized, wantAuth: 1, wantInstance: 2},
		{name: "empty origin", authorization: "Bearer token", instance: "instance", origin: fuzzEmptyPresent, wantStatus: http.StatusForbidden, wantAuth: 1, wantInstance: 1, wantOrigin: 1},
		{name: "duplicate origin", authorization: "Bearer token", instance: "instance", origin: fuzzDuplicate, wantStatus: http.StatusForbidden, wantAuth: 1, wantInstance: 1, wantOrigin: 2},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			header := make(http.Header)
			setFuzzHeader(header, "Authorization", test.authorization)
			setFuzzHeader(header, "X-BitBook-Instance", test.instance)
			setFuzzHeader(header, "Origin", test.origin)
			request := &http.Request{
				Header:     header,
				Host:       "127.0.0.1:1234",
				RemoteAddr: "127.0.0.1:9",
			}
			server := &Server{bound: "127.0.0.1:1234", token: "token", instance: "instance"}
			if got := localRequestAuthorized(server, request); got != test.wantStatus {
				t.Errorf("authorization status=%d want=%d headers=%v", got, test.wantStatus, header)
			}
			if got := len(header.Values("Authorization")); got != test.wantAuth {
				t.Errorf("Authorization values=%d want=%d raw=%v", got, test.wantAuth, header)
			}
			instances := header.Values("X-BitBook-Instance")
			if len(instances) != test.wantInstance {
				t.Errorf("instance values=%d want=%d raw=%v", len(instances), test.wantInstance, header)
			}
			if test.name == "valid" && (len(instances) != 1 || instances[0] != "instance") {
				t.Errorf("valid instance values=%q, want exactly [instance]", instances)
			}
			if got := len(header.Values("Origin")); got != test.wantOrigin {
				t.Errorf("Origin values=%d want=%d raw=%v", got, test.wantOrigin, header)
			}
		})
	}
}

func FuzzMEDIA001M2CRequest(f *testing.F) {
	validReference := "01010101010101010101010101010101"
	validHandle := "02020202020202020202020202020202"
	f.Add("GET", "/v1/media/public/"+validReference, "127.0.0.1:9", "127.0.0.1:1234", "Bearer token", "instance", "", "")
	f.Add("PUT", "/v1/media/public/"+validReference+"?", "127.0.0.1:9", "127.0.0.1:1234", "Bearer token", "instance", "", "L=64;bytes=0-1")
	f.Add("GET", "/v1/media/handles/"+validHandle, "192.0.2.1:9", "127.0.0.1:1234", "Bearer token", "instance", "", "L=64;bytes=0-1")
	f.Add("HEAD", "/v1/media/handles/%30", "127.0.0.1:9", "127.0.0.1:1234", "Bearer wrong", "instance", "null", "L=64;bytes=-1")
	f.Add("GET", "/v1/media/handles/"+validHandle, "127.0.0.1:9", "127.0.0.1:1234", fuzzDuplicate, "instance", "", "D=L=64;bytes=0-1")
	f.Add("GET", "/v1/media/handles/"+validHandle, "127.0.0.1:9", "127.0.0.1:1234", "Bearer token", fuzzEmptyPresent, fuzzEmptyPresent, "L=0;bytes=0-0")
	f.Add("GET", "/v1/media/handles/"+validHandle, "127.0.0.1:9", "127.0.0.1:1234", fuzzEmptyPresent, "instance", "", "L=64;bytes=0-1")
	f.Add("GET", "/v1/media/handles/"+validHandle, "127.0.0.1:9", "127.0.0.1:1234", "Bearer token", fuzzDuplicate, "", "L=64;bytes=0-1")
	f.Add("GET", "/v1/media/handles/"+validHandle, "127.0.0.1:9", "127.0.0.1:1234", "Bearer token", "instance", fuzzDuplicate, "L=64;bytes=0-1")
	f.Add("GET", "/v1/media/handles/"+validHandle, "127.0.0.1:9", "127.0.0.1:1234", "Bearer token", "instance", "", "L=8388608;bytes=0-8388607")
	f.Add("GET", "/v1/media/handles/"+validHandle, "127.0.0.1:9", "127.0.0.1:1234", "Bearer token", "instance", "", "L=8388609;bytes=0-8388608")
	f.Add("GET", "/v1/media/handles/"+validHandle, "127.0.0.1:9", "127.0.0.1:1234", "Bearer token", "instance", "", "L=64;bytes=0-")
	f.Add("GET", "/v1/media/handles/"+validHandle, "127.0.0.1:9", "127.0.0.1:1234", "Bearer token", "instance", "", "L=64;bytes=0-1,3-4")
	f.Add("GET", "/v1/media/handles/"+validHandle, "127.0.0.1:9", "127.0.0.1:1234", "Bearer token", "instance", "", "L=64;bytes=0-999999999999999999999999")
	f.Fuzz(func(t *testing.T, method, target, remote, host, authorization, instance, origin, rangeInput string) {
		for _, value := range []string{method, target, remote, host, authorization, instance, origin, rangeInput} {
			if len(value) > 512 {
				t.Skip()
			}
		}
		parsed, err := url.ParseRequestURI(target)
		if err != nil {
			parsed = &url.URL{Path: target}
		}
		req := &http.Request{
			Method:     method,
			URL:        parsed,
			RequestURI: target,
			Header:     make(http.Header),
			Host:       host,
			RemoteAddr: remote,
		}
		setFuzzHeader(req.Header, "Authorization", authorization)
		setFuzzHeader(req.Header, "X-BitBook-Instance", instance)
		setFuzzHeader(req.Header, "Origin", origin)
		rangeValues, fileLength := decodeFuzzRange(rangeInput)
		if rangeValues != nil {
			req.Header["Range"] = rangeValues
		}

		server := &Server{bound: "127.0.0.1:1234", token: "token", instance: "instance"}
		wantAuth := independentFuzzAuthorization(server, req)
		if got := localRequestAuthorized(server, req); got != wantAuth {
			t.Fatalf("authorization=%d want=%d headers=%v remote=%q host=%q", got, wantAuth, req.Header, remote, host)
		}

		wantRoute, wantRouteErr := independentFuzzRoute(req)
		gotRoute, gotRouteErr := parseMediaRoute(req)
		if (gotRouteErr != nil) != wantRouteErr || gotRoute != wantRoute {
			t.Fatalf("route=%+v err=%v want=%+v err=%t target=%q", gotRoute, gotRouteErr, wantRoute, wantRouteErr, target)
		}

		wantRange, wantRangePresent, wantRangeErr := independentFuzzRange(rangeValues, fileLength, method == http.MethodHead)
		gotRange, gotRangePresent, gotRangeErr := parseMediaRange(rangeValues, fileLength, method == http.MethodHead)
		if (gotRangeErr != nil) != wantRangeErr || gotRangePresent != wantRangePresent || gotRange != wantRange {
			t.Fatalf("range=%+v present=%t err=%v want=%+v/%t/%t values=%q length=%d head=%t", gotRange, gotRangePresent, gotRangeErr, wantRange, wantRangePresent, wantRangeErr, rangeValues, fileLength, method == http.MethodHead)
		}
		if gotRangePresent {
			if gotRange.start < 0 || gotRange.end < gotRange.start || gotRange.end >= fileLength || gotRange.end-gotRange.start+1 > 8<<20 {
				t.Fatalf("accepted unsafe range %+v for length %d", gotRange, fileLength)
			}
		}

		if wantAuth != 0 {
			body := &countingBody{reader: bytes.NewReader([]byte("denied"))}
			req.Body = body
			req.ContentLength = int64(len("denied"))
			server.media = &mediaService{handles: map[string]mediaHandle{"sentinel": {}}}
			recorder := httptest.NewRecorder()
			server.ServeHTTP(recorder, req)
			if recorder.Code != wantAuth || body.reads != 0 || len(server.media.handles) != 1 {
				t.Fatalf("denied dispatch status=%d reads=%d handles=%d want=%d/0/1", recorder.Code, body.reads, len(server.media.handles), wantAuth)
			}
		}
	})
}

func setFuzzHeader(header http.Header, name, value string) {
	canonicalName := http.CanonicalHeaderKey(name)
	switch value {
	case "":
	case fuzzDuplicate:
		duplicateValue := "duplicate"
		switch canonicalName {
		case "Authorization":
			duplicateValue = "Bearer token"
		case "X-Bitbook-Instance":
			duplicateValue = "instance"
		}
		header[canonicalName] = []string{duplicateValue, duplicateValue}
	case fuzzEmptyPresent:
		header[canonicalName] = []string{""}
	default:
		header.Set(canonicalName, value)
	}
}

func independentFuzzAuthorization(server *Server, request *http.Request) int {
	host, _, err := net.SplitHostPort(request.RemoteAddr)
	address := net.ParseIP(host)
	if err != nil || address == nil || !address.IsLoopback() || request.Host != server.bound {
		return http.StatusForbidden
	}
	if len(request.Header.Values("Origin")) != 0 {
		return http.StatusForbidden
	}
	auth := request.Header.Values("Authorization")
	instances := request.Header.Values("X-BitBook-Instance")
	if len(auth) != 1 || auth[0] != "Bearer "+server.token || len(instances) != 1 || instances[0] != server.instance {
		return http.StatusUnauthorized
	}
	return 0
}

func independentFuzzRoute(request *http.Request) (mediaRoute, bool) {
	if request == nil || request.URL == nil {
		return mediaRoute{}, false
	}
	path := request.URL.Path
	if !strings.HasPrefix(path, "/v1/media") {
		return mediaRoute{}, false
	}
	if request.URL.RawQuery != "" || request.URL.ForceQuery || request.URL.Fragment != "" || request.URL.RawPath != "" ||
		strings.Contains(path, `\`) || strings.Contains(path, "//") || strings.Contains(path, "/./") || strings.Contains(path, "/../") ||
		strings.Contains(request.RequestURI, "%") || strings.Contains(request.RequestURI, "#") {
		return mediaRoute{}, true
	}
	parts := strings.Split(path, "/")
	if len(parts) == 5 && parts[1] == "v1" && parts[2] == "media" && parts[3] == "public" && independentFuzzID(parts[4]) {
		return mediaRoute{kind: mediaRouteReference, referenceID: parts[4]}, false
	}
	if len(parts) == 6 && parts[1] == "v1" && parts[2] == "media" && parts[3] == "public" && independentFuzzID(parts[4]) && parts[5] == "handles" {
		return mediaRoute{kind: mediaRouteCreateHandle, referenceID: parts[4]}, false
	}
	if len(parts) == 5 && parts[1] == "v1" && parts[2] == "media" && parts[3] == "handles" && independentFuzzID(parts[4]) {
		return mediaRoute{kind: mediaRouteDownload, handle: parts[4]}, false
	}
	return mediaRoute{}, true
}

func independentFuzzID(value string) bool {
	if len(value) != 32 || value == "00000000000000000000000000000000" {
		return false
	}
	for index := 0; index < len(value); index++ {
		character := value[index]
		if !((character >= '0' && character <= '9') || (character >= 'a' && character <= 'f')) {
			return false
		}
	}
	return true
}

func decodeFuzzRange(input string) ([]string, int64) {
	length := int64(64)
	duplicate := false
	if strings.HasPrefix(input, "D=") {
		duplicate = true
		input = input[2:]
	}
	if strings.HasPrefix(input, "L=") {
		if separator := strings.IndexByte(input, ';'); separator >= 2 {
			if parsed, err := strconv.ParseUint(input[2:separator], 10, 27); err == nil && parsed <= 100<<20+1 {
				length = int64(parsed)
				input = input[separator+1:]
			}
		}
	}
	if input == "" {
		return nil, length
	}
	if duplicate {
		return []string{input, input}, length
	}
	return []string{input}, length
}

func independentFuzzRange(values []string, length int64, head bool) (mediaByteRange, bool, bool) {
	if len(values) == 0 {
		return mediaByteRange{}, false, false
	}
	if len(values) != 1 || head || length == 0 {
		return mediaByteRange{}, false, true
	}
	value := values[0]
	if !strings.HasPrefix(value, "bytes=") || strings.Contains(value, ",") {
		return mediaByteRange{}, false, true
	}
	parts := strings.Split(value[len("bytes="):], "-")
	if len(parts) != 2 || parts[0] == "" || parts[1] == "" {
		return mediaByteRange{}, false, true
	}
	start, startOK := independentFuzzDecimal(parts[0])
	end, endOK := independentFuzzDecimal(parts[1])
	if !startOK || !endOK || start > end || end >= uint64(length) || end-start+1 > 8<<20 {
		return mediaByteRange{}, false, true
	}
	return mediaByteRange{start: int64(start), end: int64(end)}, true, false
}

func independentFuzzDecimal(value string) (uint64, bool) {
	if value == "" {
		return 0, false
	}
	for index := 0; index < len(value); index++ {
		if value[index] < '0' || value[index] > '9' {
			return 0, false
		}
	}
	parsed, err := strconv.ParseUint(value, 10, 63)
	return parsed, err == nil
}
