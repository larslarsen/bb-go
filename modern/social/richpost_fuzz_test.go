package social

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"testing"
	"time"

	blocks "github.com/ipfs/go-block-format"
	"github.com/libp2p/go-libp2p/core/peer"
)

func FuzzMEDIA001M1BRichPost(f *testing.F) {
	key := deterministicRichKey(f)
	author, err := peer.IDFromPrivateKey(key)
	if err != nil {
		f.Fatal(err)
	}
	valid := []byte(`{"schema":"bitbook.public-post/1","id":"11111111111111111111111111111111","vendorID":{"peerID":"` + author.String() + `"},"timestamp":"2026-09-20T00:00:00Z","slug":"rich-11111111111111111111111111111111","postType":"POST","status":"x","content":{"schema":"bitbook.public-content/1","body":[{"type":"paragraph","children":[{"type":"text","text":"x","marks":[],"href":""}]}],"attachments":[]}}`)
	f.Add(valid)
	f.Add([]byte(`{"schema":"bitbook.public-post/1","schema":"bitbook.public-post/1"}`))
	f.Add(bytes.Replace(valid, []byte(`{`), []byte(`{"ignored":1e9999,`), 1))
	f.Add([]byte{0xff, 0xfe})
	f.Fuzz(func(t *testing.T, raw []byte) {
		if len(raw) > maxRichPayloadBytes+1 {
			t.Skip()
		}
		parsed, err := parseRichPayload(author, raw)
		post := signedFixtureWithoutCID(t, key, raw)
		verifyErr := VerifyPost(author, post)
		candidate, classificationErr := classifyRichCandidate(raw)
		if classificationErr != nil || candidate {
			if err == nil && verifyErr != nil {
				t.Fatalf("valid parsed rich post rejected by VerifyPost: %v", verifyErr)
			}
			if err != nil && !errors.Is(verifyErr, ErrInvalidRichPost) {
				t.Fatalf("invalid rich candidate verification error=%v", verifyErr)
			}
		} else if verifyErr != nil {
			t.Fatalf("legacy signature path rejected: %v", verifyErr)
		}
		if bytes.Equal(raw, valid) && err != nil {
			t.Fatalf("valid seed rejected: %v", err)
		}
		if err != nil {
			return
		}
		if parsed.ID == "" || parsed.Author != author || parsed.Timestamp.Before(time.Unix(0, 0)) {
			t.Fatalf("accepted invalid payload: %+v", parsed)
		}
		canonical, err := encodeRichPayload(parsed)
		if err != nil || !bytes.Equal(canonical, raw) {
			t.Fatalf("canonical mismatch err=%v\n got=%s\nwant=%s", err, canonical, raw)
		}
	})
}

func FuzzMEDIA001M1BRecord(f *testing.F) {
	valid := []byte(`{"version":1,"state":"deleted","id":"11111111111111111111111111111111","requestDigest":"aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa"}`)
	postCID := blocks.NewBlock([]byte("fuzz live envelope")).Cid().String()
	claimCID := blocks.NewBlock([]byte("fuzz live claim")).Cid().String()
	live := []byte(fmt.Sprintf(`{"version":1,"state":"live","id":"22222222222222222222222222222222","requestDigest":"bbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbb","envelope":{"post":null,"signature":null,"publicKey":null,"hash":%q},"claims":[{"attachmentId":"aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa","uploadId":"01010101010101010101010101010101","targetId":"02020202020202020202020202020202","cid":%q,"byteLength":15}],"claimSignature":"AQ=="}`, postCID, claimCID))
	f.Add(valid)
	f.Add(live)
	f.Add([]byte(`{"version":1,"version":1}`))
	f.Add([]byte(`null`))
	f.Fuzz(func(t *testing.T, raw []byte) {
		if len(raw) > maxRichRecordBytes+1 {
			t.Skip()
		}
		record, err := decodeRichRecord(raw)
		if bytes.Equal(raw, valid) && err != nil {
			t.Fatalf("valid seed rejected: %v", err)
		}
		if err != nil {
			return
		}
		encoded, err := encodeRichRecord(record)
		if err != nil || !bytes.Equal(encoded, raw) {
			t.Fatalf("record instability err=%v\n got=%s\nwant=%s", err, encoded, raw)
		}
		var object map[string]json.RawMessage
		if err := json.Unmarshal(encoded, &object); err != nil || len(object) < 4 {
			t.Fatalf("accepted record shape fields=%d err=%v", len(object), err)
		}
	})
}
