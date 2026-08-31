package payment

import (
	"bytes"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
)

const copiedGoldenSHA256 = "08905d2b082fa6370211ebd494130d62e3696db0f8314636f3685ba5887f929f"

type goldenFile struct {
	Vectors        []goldenVector `json:"vectors"`
	InvalidVectors []goldenVector `json:"invalid_vectors"`
}

type goldenVector struct {
	Name            string          `json:"name"`
	Kind            string          `json:"kind"`
	Classification  string          `json:"classification"`
	DomainSeparator string          `json:"domain_separator"`
	Input           json.RawMessage `json:"input"`
	Canonical       string          `json:"canonical"`
	Digest          string          `json:"digest"`
	ExpectedCode    string          `json:"expected_code"`
	RawJSON         string          `json:"raw_json"`
	HexBytes        string          `json:"hex_bytes"`
	Reason          string          `json:"reason"`
}

func TestCopiedFixtureMatchesDesktopOracle(t *testing.T) {
	raw := readCopiedGolden(t)
	sum := sha256.Sum256(raw)
	got := hex.EncodeToString(sum[:])
	if got != copiedGoldenSHA256 {
		t.Fatalf("copied fixture SHA-256 = %s, want %s", got, copiedGoldenSHA256)
	}
	if n := bytes.Count(raw, []byte("\n")); n != 231 {
		t.Fatalf("copied fixture newlines = %d, want 231", n)
	}
}

func TestGoldenValidPaymentObjectsMatchCanonicalAndDigest(t *testing.T) {
	golden := loadGolden(t)
	var sawRequest, sawStatus bool
	for _, vector := range golden.Vectors {
		if vector.Kind != "payment_request_v1" && vector.Kind != "payment_status_event_v1" {
			continue
		}
		t.Run(vector.Name, func(t *testing.T) {
			canonical, digest := decodeGoldenPayment(t, vector)
			if string(canonical) != vector.Canonical {
				t.Fatalf("canonical bytes = %s, want %s", canonical, vector.Canonical)
			}
			if digest != vector.Digest {
				t.Fatalf("digest = %s, want %s", digest, vector.Digest)
			}
			if err := CheckCanonicalCopy(canonical, []byte(vector.Canonical)); err != nil {
				t.Fatalf("fixture canonical copy rejected: %v", err)
			}
			if vector.Kind == "payment_request_v1" {
				sawRequest = true
				if vector.DomainSeparator != DomainSeparatorRequest {
					t.Fatalf("domain separator = %q, want %q", vector.DomainSeparator, DomainSeparatorRequest)
				}
			}
			if vector.Kind == "payment_status_event_v1" {
				sawStatus = true
				if vector.DomainSeparator != DomainSeparatorStatus {
					t.Fatalf("domain separator = %q, want %q", vector.DomainSeparator, DomainSeparatorStatus)
				}
			}
		})
	}
	if !sawRequest || !sawStatus {
		t.Fatalf("fixture missing accepted request/status vectors: request=%t status=%t", sawRequest, sawStatus)
	}
}

func TestGoldenInvalidPaymentObjectsFailWithNamedCode(t *testing.T) {
	golden := loadGolden(t)
	seen := map[string]bool{}
	for _, vector := range golden.InvalidVectors {
		if vector.Kind != "payment_request_v1" && vector.Kind != "payment_status_event_v1" {
			continue
		}
		t.Run(vector.Name, func(t *testing.T) {
			raw := goldenInvalidBytes(t, vector)
			_, _, _, err := decodePayment(vector.Kind, raw)
			if err == nil {
				t.Fatalf("invalid vector %s was accepted (%s)", vector.Name, vector.Reason)
			}
			if codeOf(err) != vector.ExpectedCode {
				t.Fatalf("code = %q, want %q (%v)", codeOf(err), vector.ExpectedCode, err)
			}
			seen[vector.Name] = true
		})
	}
	required := []string{
		"payment-request-duplicate-key",
		"payment-request-malformed-utf8-memo",
		"payment-request-unknown-field",
		"payment-request-missing-memo",
		"payment-request-zero-amount",
		"payment-request-leading-zero-amount",
		"payment-request-scientific-amount",
		"payment-request-invented-ironwood-receiver",
		"payment-request-status-field",
		"payment-request-impossible-calendar-date",
		"payment-request-out-of-range-timestamp",
		"payment-request-non-nfc-memo",
		"payment-request-bidi-control",
		"payment-request-format-control",
		"payment-status-paid-without-tx-ref",
		"payment-request-rate-field",
	}
	for _, name := range required {
		if !seen[name] {
			t.Fatalf("fixture-invalid payment vector %s was not exercised", name)
		}
	}
}

func TestTimestampBoundaries(t *testing.T) {
	cases := []struct {
		name      string
		created   string
		expires   string
		wantCode  string
		wantValid bool
	}{
		{name: "february30Midnight", created: "2026-02-30T00:00:00Z", expires: "2026-08-30T12:15:00Z", wantCode: CodeSchema},
		{name: "hour24", created: "2026-01-01T24:00:00Z", expires: "2026-08-30T12:15:00Z", wantCode: CodeSchema},
		{name: "year0000", created: "0000-01-01T00:00:00Z", expires: "2026-08-30T12:15:00Z", wantCode: CodeSchema},
		{name: "nonLeapFebruary29", created: "2026-02-29T00:00:00Z", expires: "2026-08-30T12:15:00Z", wantCode: CodeSchema},
		{name: "april31", created: "2026-04-31T00:00:00Z", expires: "2026-08-30T12:15:00Z", wantCode: CodeSchema},
		{name: "month13", created: "2026-13-01T00:00:00Z", expires: "2026-08-30T12:15:00Z", wantCode: CodeSchema},
		{name: "year2101", created: "2101-01-01T00:00:00Z", expires: "2101-01-01T00:15:00Z", wantCode: CodeSchema},
		{name: "year2019", created: "2019-12-31T23:59:59Z", expires: "2026-08-30T12:15:00Z", wantCode: CodeSchema},
		{name: "offsetPlusZero", created: "2026-08-30T12:00:00+00:00", expires: "2026-08-30T12:15:00Z", wantCode: CodeSchema},
		{name: "fractionalSeconds", created: "2026-08-30T12:00:00.0Z", expires: "2026-08-30T12:15:00Z", wantCode: CodeSchema},
		{name: "leapSecond", created: "2026-08-30T12:00:60Z", expires: "2026-08-30T12:15:00Z", wantCode: CodeSchema},
		{name: "expiresEqualCreated", created: "2026-08-30T12:00:00Z", expires: "2026-08-30T12:00:00Z", wantCode: CodeSchema},
		{name: "expiresBeforeCreated", created: "2026-08-30T12:15:00Z", expires: "2026-08-30T12:00:00Z", wantCode: CodeSchema},
		{name: "leapDay2024", created: "2024-02-29T12:00:00Z", expires: "2024-02-29T12:15:00Z", wantValid: true},
		{name: "year2020LowerBound", created: "2020-01-01T00:00:00Z", expires: "2020-01-01T00:00:01Z", wantValid: true},
		{name: "year2100UpperBound", created: "2100-12-31T23:59:58Z", expires: "2100-12-31T23:59:59Z", wantValid: true},
	}
	for _, test := range cases {
		t.Run(test.name, func(t *testing.T) {
			req := baseRequest()
			req["created_at"] = test.created
			req["expires_at"] = test.expires
			_, _, _, err := DecodePaymentRequest(mustJSON(t, req))
			if test.wantValid {
				if err != nil {
					t.Fatalf("valid timestamp rejected: %v", err)
				}
				return
			}
			if err == nil {
				t.Fatal("invalid timestamp was accepted")
			}
			if codeOf(err) != test.wantCode {
				t.Fatalf("code = %q, want %q (%v)", codeOf(err), test.wantCode, err)
			}
		})
	}
}

func TestUnicodePolicy(t *testing.T) {
	cases := []struct {
		name string
		memo string
	}{
		{name: "ltrEmbedding", memo: "safe\u202Atext"},
		{name: "rtlEmbedding", memo: "safe\u202Btext"},
		{name: "popDirectional", memo: "safe\u202Ctext"},
		{name: "ltrOverride", memo: "safe\u202Dtext"},
		{name: "rtlOverride", memo: "safe\u202Etext"},
		{name: "ltrIsolate", memo: "safe\u2066text"},
		{name: "rtlIsolate", memo: "safe\u2067text"},
		{name: "firstStrongIsolate", memo: "safe\u2068text"},
		{name: "popIsolate", memo: "safe\u2069text"},
		{name: "zeroWidthSpace", memo: "safe\u200Btext"},
		{name: "zeroWidthNonJoiner", memo: "safe\u200Ctext"},
		{name: "zeroWidthJoiner", memo: "safe\u200Dtext"},
		{name: "ltrMark", memo: "safe\u200Etext"},
		{name: "rtlMark", memo: "safe\u200Ftext"},
		{name: "arabicLetterMark", memo: "safe\u061Ctext"},
		{name: "wordJoiner", memo: "safe\u2060text"},
		{name: "deprecatedFormat206A", memo: "safe\u206Atext"},
		{name: "byteOrderMark", memo: "safe\uFEFFtext"},
		{name: "interlinearAnnotation", memo: "safe\uFFF9text"},
		{name: "tagCharacter", memo: "safe\U000E0001text"},
		{name: "c0Control", memo: "safe\u0001text"},
		{name: "delete", memo: "safe\u007Ftext"},
		{name: "c1Control", memo: "safe\u0080text"},
		{name: "noncharacterFDD0", memo: "safe\uFDD0text"},
		{name: "noncharacterFFFE", memo: "safe\uFFFEtext"},
		{name: "nonNFC", memo: "cafe\u0301"},
		{name: "memo513Bytes", memo: strings.Repeat("a", 513)},
	}
	for _, test := range cases {
		t.Run(test.name, func(t *testing.T) {
			req := baseRequest()
			req["memo"] = test.memo
			_, _, _, err := DecodePaymentRequest(mustJSON(t, req))
			if err == nil {
				t.Fatal("forbidden unicode was accepted")
			}
			if codeOf(err) != CodeSchema {
				t.Fatalf("code = %q, want %q (%v)", codeOf(err), CodeSchema, err)
			}
		})
	}
	t.Run("emptyMemoAllowed", func(t *testing.T) {
		req := baseRequest()
		req["memo"] = ""
		if _, _, _, err := DecodePaymentRequest(mustJSON(t, req)); err != nil {
			t.Fatalf("empty memo rejected: %v", err)
		}
	})
	t.Run("memo512BytesAllowed", func(t *testing.T) {
		req := baseRequest()
		req["memo"] = strings.Repeat("a", 512)
		if _, _, _, err := DecodePaymentRequest(mustJSON(t, req)); err != nil {
			t.Fatalf("512-byte memo rejected: %v", err)
		}
	})
	t.Run("bomPrefixRejected", func(t *testing.T) {
		raw := append([]byte{0xEF, 0xBB, 0xBF}, mustJSON(t, baseRequest())...)
		_, _, _, err := DecodePaymentRequest(raw)
		if codeOf(err) != CodeSchema {
			t.Fatalf("BOM prefix code = %q (%v)", codeOf(err), err)
		}
	})
}

func TestTypeAndClosedSchemaBoundaries(t *testing.T) {
	cases := []struct {
		name string
		raw  string
	}{
		{name: "versionString", raw: replaceRequestJSON(t, `"v":1`, `"v":"1"`)},
		{name: "versionTwo", raw: replaceRequestJSON(t, `"v":1`, `"v":2`)},
		{name: "versionFloat", raw: replaceRequestJSON(t, `"v":1`, `"v":1.0`)},
		{name: "versionTrue", raw: replaceRequestJSON(t, `"v":1`, `"v":true`)},
		{name: "amountNumber", raw: replaceRequestJSON(t, `"amount_atomic":"100000000"`, `"amount_atomic":100000000`)},
		{name: "memoNull", raw: replaceRequestJSON(t, `"memo":"coffee"`, `"memo":null`)},
		{name: "trailingData", raw: string(mustJSON(t, baseRequest())) + `{}`},
		{name: "jsonArray", raw: `[]`},
		{name: "emptyObject", raw: `{}`},
		{name: "emptyKey", raw: replaceRequestJSON(t, `"memo":"coffee"`, `"":"x","memo":"coffee"`)},
		{name: "missingRequestID", raw: string(mustJSON(t, withoutKey(baseRequest(), "request_id")))},
		{name: "cborBytes", raw: string([]byte{0xa1, 0x61, 0x76, 0x01})},
		{name: "peerIDUnicode", raw: string(mustJSON(t, withField(baseRequest(), "payer_peer_id", "12D3KooWPayér")))},
		{name: "uppercaseHexID", raw: string(mustJSON(t, withField(baseRequest(), "request_id", "00112233445566778899AABBCCDDEEFF")))},
		{name: "shortHexID", raw: string(mustJSON(t, withField(baseRequest(), "nonce", "ffeeddccbbaa9988776655443322110")))},
		{name: "emptyPayer", raw: string(mustJSON(t, withField(baseRequest(), "payer_peer_id", "")))},
	}
	for _, test := range cases {
		t.Run(test.name, func(t *testing.T) {
			_, _, _, err := DecodePaymentRequest([]byte(test.raw))
			if err == nil {
				t.Fatal("invalid type/schema input was accepted")
			}
			if codeOf(err) != CodeSchema {
				t.Fatalf("code = %q, want %q (%v)", codeOf(err), CodeSchema, err)
			}
		})
	}
}

func TestAmountBoundaries(t *testing.T) {
	cases := []struct {
		name      string
		amount    string
		wantValid bool
	}{
		{name: "oneAtomic", amount: "1", wantValid: true},
		{name: "twentyDigits", amount: "12345678901234567890", wantValid: true},
		{name: "zero", amount: "0"},
		{name: "leadingZero", amount: "01"},
		{name: "scientific", amount: "1e8"},
		{name: "decimal", amount: "1.0"},
		{name: "grouped", amount: "1,000"},
		{name: "negative", amount: "-1"},
		{name: "empty", amount: ""},
		{name: "twentyOneDigits", amount: "123456789012345678901"},
		{name: "plusSign", amount: "+1"},
	}
	for _, test := range cases {
		t.Run(test.name, func(t *testing.T) {
			req := baseRequest()
			req["amount_atomic"] = test.amount
			_, _, _, err := DecodePaymentRequest(mustJSON(t, req))
			if test.wantValid {
				if err != nil {
					t.Fatalf("valid amount rejected: %v", err)
				}
				return
			}
			if codeOf(err) != CodeSchema {
				t.Fatalf("code = %q, want %q (%v)", codeOf(err), CodeSchema, err)
			}
		})
	}
}

func TestNetworkAndReceiverKindBoundaries(t *testing.T) {
	cases := []struct {
		name         string
		asset        string
		network      string
		receiverKind string
		receiver     string
		wantValid    bool
	}{
		{name: "validZECTestnet", asset: "ZEC", network: "zec-testnet", receiverKind: "zec-ua-orchard-protocol", receiver: "u1testreceiver", wantValid: true},
		{name: "validZECRegtest", asset: "ZEC", network: "zec-regtest", receiverKind: "zec-ua-orchard-protocol", receiver: "u1testreceiver", wantValid: true},
		{name: "validXMRStagenet", asset: "XMR", network: "xmr-stagenet", receiverKind: "xmr-subaddress", receiver: "73testsubaddress", wantValid: true},
		{name: "validXMRMainnet", asset: "XMR", network: "xmr-mainnet", receiverKind: "xmr-subaddress", receiver: "4testsubaddress", wantValid: true},
		{name: "validXMRTestnet", asset: "XMR", network: "xmr-testnet", receiverKind: "xmr-subaddress", receiver: "53testsubaddress", wantValid: true},
		{name: "validZECMainnet", asset: "ZEC", network: "zec-mainnet", receiverKind: "zec-ua-orchard-protocol", receiver: "u1testreceiver", wantValid: true},
		{name: "zecWithXMRNetwork", asset: "ZEC", network: "xmr-stagenet", receiverKind: "zec-ua-orchard-protocol", receiver: "u1testreceiver"},
		{name: "xmrWithZECNetwork", asset: "XMR", network: "zec-testnet", receiverKind: "xmr-subaddress", receiver: "73testsubaddress"},
		{name: "zecWithXMRReceiverKind", asset: "ZEC", network: "zec-testnet", receiverKind: "xmr-subaddress", receiver: "u1testreceiver"},
		{name: "xmrWithZECReceiverKind", asset: "XMR", network: "xmr-stagenet", receiverKind: "zec-ua-orchard-protocol", receiver: "73testsubaddress"},
		{name: "inventedIronwoodKind", asset: "ZEC", network: "zec-testnet", receiverKind: "zec-ua-ironwood", receiver: "u1testreceiver"},
		{name: "unknownAsset", asset: "BTC", network: "zec-testnet", receiverKind: "zec-ua-orchard-protocol", receiver: "u1testreceiver"},
		{name: "unknownNetwork", asset: "ZEC", network: "zec-signet", receiverKind: "zec-ua-orchard-protocol", receiver: "u1testreceiver"},
		{name: "unicodeReceiver", asset: "ZEC", network: "zec-testnet", receiverKind: "zec-ua-orchard-protocol", receiver: "u1tést"},
	}
	for _, test := range cases {
		t.Run(test.name, func(t *testing.T) {
			req := baseRequest()
			req["asset"] = test.asset
			req["network"] = test.network
			req["receiver_kind"] = test.receiverKind
			req["receiver"] = test.receiver
			_, _, _, err := DecodePaymentRequest(mustJSON(t, req))
			if test.wantValid {
				if err != nil {
					t.Fatalf("valid network relation rejected: %v", err)
				}
				return
			}
			if codeOf(err) != CodeSchema {
				t.Fatalf("code = %q, want %q (%v)", codeOf(err), CodeSchema, err)
			}
		})
	}
}

func TestStatusTxRefBoundaries(t *testing.T) {
	cases := []struct {
		name      string
		status    string
		txRef     string
		omitTxRef bool
		wantValid bool
	}{
		{name: "cancelledEmptyTxRef", status: StatusCancelled, txRef: "", wantValid: true},
		{name: "paidWithTxRef", status: StatusPaid, txRef: "abc123", wantValid: true},
		{name: "expiredEmptyTxRef", status: StatusExpired, txRef: "", wantValid: true},
		{name: "paidEmptyTxRef", status: StatusPaid, txRef: ""},
		{name: "cancelledNonEmptyTxRef", status: StatusCancelled, txRef: "abc123"},
		{name: "expiredNonEmptyTxRef", status: StatusExpired, txRef: "abc123"},
		{name: "unknownStatus", status: "open", txRef: ""},
		{name: "missingTxRefKey", status: StatusCancelled, omitTxRef: true},
	}
	for _, test := range cases {
		t.Run(test.name, func(t *testing.T) {
			event := baseStatus()
			event["status"] = test.status
			if test.omitTxRef {
				delete(event, "tx_ref")
			} else {
				event["tx_ref"] = test.txRef
			}
			_, _, _, err := DecodePaymentStatus(mustJSON(t, event))
			if test.wantValid {
				if err != nil {
					t.Fatalf("valid status shape rejected: %v", err)
				}
				return
			}
			if codeOf(err) != CodeSchema {
				t.Fatalf("code = %q, want %q (%v)", codeOf(err), CodeSchema, err)
			}
		})
	}
}

func TestStatusClosedSchemaBoundaries(t *testing.T) {
	baseline := mustJSON(t, baseStatus())
	paid := withField(withField(baseStatus(), "status", StatusPaid), "tx_ref", "txid")
	cases := []struct {
		name string
		raw  []byte
	}{
		{name: "duplicateKey", raw: []byte(replaceStatusJSON(t, `"v":1`, `"v":1,"v":1`))},
		{name: "unknownKey", raw: mustJSON(t, withField(baseStatus(), "unknown", "closed-schema"))},
		{name: "missingVersion", raw: mustJSON(t, withoutKey(baseStatus(), "v"))},
		{name: "missingRequestID", raw: mustJSON(t, withoutKey(baseStatus(), "request_id"))},
		{name: "missingEventID", raw: mustJSON(t, withoutKey(baseStatus(), "event_id"))},
		{name: "missingNonce", raw: mustJSON(t, withoutKey(baseStatus(), "nonce"))},
		{name: "missingStatus", raw: mustJSON(t, withoutKey(baseStatus(), "status"))},
		{name: "missingAt", raw: mustJSON(t, withoutKey(baseStatus(), "at"))},
		{name: "missingTxRef", raw: mustJSON(t, withoutKey(baseStatus(), "tx_ref"))},
		{name: "versionTwo", raw: mustJSON(t, withField(baseStatus(), "v", 2))},
		{name: "versionString", raw: mustJSON(t, withField(baseStatus(), "v", "1"))},
		{name: "versionFloat", raw: []byte(replaceStatusJSON(t, `"v":1`, `"v":1.0`))},
		{name: "statusBoolean", raw: mustJSON(t, withField(baseStatus(), "status", true))},
		{name: "txRefNull", raw: mustJSON(t, withField(baseStatus(), "tx_ref", nil))},
		{name: "shortRequestID", raw: mustJSON(t, withField(baseStatus(), "request_id", "00112233445566778899aabbccddeef"))},
		{name: "uppercaseRequestID", raw: mustJSON(t, withField(baseStatus(), "request_id", "00112233445566778899AABBCCDDEEFF"))},
		{name: "shortEventID", raw: mustJSON(t, withField(baseStatus(), "event_id", "1111222233334444555566667777888"))},
		{name: "uppercaseEventID", raw: mustJSON(t, withField(baseStatus(), "event_id", "1111222233334444555566667777888A"))},
		{name: "shortNonce", raw: mustJSON(t, withField(baseStatus(), "nonce", "9999aaaabbbbccccddddeeeeffff000"))},
		{name: "uppercaseNonce", raw: mustJSON(t, withField(baseStatus(), "nonce", "9999AAAABBBBCCCCDDDDEEEEFFFF0000"))},
		{name: "offsetTimestamp", raw: mustJSON(t, withField(baseStatus(), "at", "2026-08-30T12:05:00+00:00"))},
		{name: "fractionalTimestamp", raw: mustJSON(t, withField(baseStatus(), "at", "2026-08-30T12:05:00.0Z"))},
		{name: "impossibleDate", raw: mustJSON(t, withField(baseStatus(), "at", "2026-02-30T12:05:00Z"))},
		{name: "hour24", raw: mustJSON(t, withField(baseStatus(), "at", "2026-08-30T24:00:00Z"))},
		{name: "belowYearRange", raw: mustJSON(t, withField(baseStatus(), "at", "2019-12-31T23:59:59Z"))},
		{name: "aboveYearRange", raw: mustJSON(t, withField(baseStatus(), "at", "2101-01-01T00:00:00Z"))},
		{name: "nonASCIIStatus", raw: mustJSON(t, withField(baseStatus(), "status", "cancelléd"))},
		{name: "controlStatus", raw: mustJSON(t, withField(baseStatus(), "status", "cancelled\u0001"))},
		{name: "nonASCIIRequestID", raw: mustJSON(t, withField(baseStatus(), "request_id", "00112233445566778899aabbccddeefé"))},
		{name: "controlRequestID", raw: mustJSON(t, withField(baseStatus(), "request_id", "00112233445566778899aabbccddeef\u0001"))},
		{name: "nonASCIIEventID", raw: mustJSON(t, withField(baseStatus(), "event_id", "111122223333444455556666777788é"))},
		{name: "controlEventID", raw: mustJSON(t, withField(baseStatus(), "event_id", "111122223333444455556666777788\u0001"))},
		{name: "nonASCIINonce", raw: mustJSON(t, withField(baseStatus(), "nonce", "9999aaaabbbbccccddddeeeeffff000é"))},
		{name: "controlNonce", raw: mustJSON(t, withField(baseStatus(), "nonce", "9999aaaabbbbccccddddeeeeffff000\u0001"))},
		{name: "nonASCIITimestamp", raw: mustJSON(t, withField(baseStatus(), "at", "2026-08-30T12:05:00é"))},
		{name: "controlTimestamp", raw: mustJSON(t, withField(baseStatus(), "at", "2026-08-30T12:05:00\u0001"))},
		{name: "txRefNonASCII", raw: mustJSON(t, withField(paid, "tx_ref", "txréf"))},
		{name: "txRefC0Control", raw: mustJSON(t, withField(paid, "tx_ref", "tx\u0001ref"))},
		{name: "txRefBidiControl", raw: mustJSON(t, withField(paid, "tx_ref", "tx\u202eref"))},
	}
	for _, test := range cases {
		t.Run(test.name, func(t *testing.T) {
			if bytes.Equal(test.raw, baseline) {
				t.Fatal("status mutation did not change the baseline bytes")
			}
			_, _, _, err := DecodePaymentStatus(test.raw)
			if codeOf(err) != CodeSchema {
				t.Fatalf("code = %q, want %q (%v)", codeOf(err), CodeSchema, err)
			}
		})
	}
}

func TestRateAndQuoteFieldsRejected(t *testing.T) {
	for _, field := range []string{"rate", "quote", "fiat", "fiat_estimate", "amount_usd"} {
		t.Run(field, func(t *testing.T) {
			req := baseRequest()
			req[field] = "1.25"
			_, _, _, err := DecodePaymentRequest(mustJSON(t, req))
			if codeOf(err) != CodeSchema {
				t.Fatalf("field %s code = %q (%v)", field, codeOf(err), err)
			}
		})
	}
}

func TestCanonicalCopyMustEqualJCS(t *testing.T) {
	_, canonical, _, err := DecodePaymentRequest(mustJSON(t, baseRequest()))
	if err != nil {
		t.Fatal(err)
	}
	if err := CheckCanonicalCopy(canonical, canonical); err != nil {
		t.Fatalf("exact JCS copy rejected: %v", err)
	}
	spaced := []byte(`{ "amount_atomic": "100000000", "asset": "ZEC" }`)
	if err := CheckCanonicalCopy(canonical, spaced); codeOf(err) != CodeSchema {
		t.Fatalf("whitespace copy code = %q (%v)", codeOf(err), err)
	}
	reordered := []byte(`{"v":1,"amount_atomic":"100000000"}`)
	if err := CheckCanonicalCopy(canonical, reordered); codeOf(err) != CodeSchema {
		t.Fatalf("reordered copy code = %q (%v)", codeOf(err), err)
	}
}

func TestJCSIsDeterministicAcrossKeyOrderAndWhitespace(t *testing.T) {
	compact := mustJSON(t, baseRequest())
	pretty, err := json.MarshalIndent(baseRequest(), "", "  ")
	if err != nil {
		t.Fatal(err)
	}
	reordered := []byte(`{"v":1,"expires_at":"2026-08-30T12:15:00Z","created_at":"2026-08-30T12:00:00Z","nonce":"ffeeddccbbaa99887766554433221100","memo":"coffee","receiver_kind":"zec-ua-orchard-protocol","receiver":"u1testreceiver","amount_atomic":"100000000","network":"zec-testnet","asset":"ZEC","payee_peer_id":"12D3KooWPayee","payer_peer_id":"12D3KooWPayer","request_id":"00112233445566778899aabbccddeeff"}`)
	want := `{"amount_atomic":"100000000","asset":"ZEC","created_at":"2026-08-30T12:00:00Z","expires_at":"2026-08-30T12:15:00Z","memo":"coffee","network":"zec-testnet","nonce":"ffeeddccbbaa99887766554433221100","payee_peer_id":"12D3KooWPayee","payer_peer_id":"12D3KooWPayer","receiver":"u1testreceiver","receiver_kind":"zec-ua-orchard-protocol","request_id":"00112233445566778899aabbccddeeff","v":1}`
	for _, name := range []struct {
		name string
		raw  []byte
	}{
		{name: "compact", raw: compact},
		{name: "pretty", raw: pretty},
		{name: "reordered", raw: reordered},
	} {
		t.Run(name.name, func(t *testing.T) {
			canonical, err := CanonicalJSON(name.raw)
			if err != nil {
				t.Fatal(err)
			}
			if string(canonical) != want {
				t.Fatalf("canonical = %s, want %s", canonical, want)
			}
			again, err := CanonicalJSON(canonical)
			if err != nil {
				t.Fatal(err)
			}
			if !bytes.Equal(canonical, again) {
				t.Fatal("JCS is not idempotent")
			}
			_, decoded, digest, err := DecodePaymentRequest(name.raw)
			if err != nil {
				t.Fatal(err)
			}
			if !bytes.Equal(decoded, canonical) {
				t.Fatal("decoder canonical diverged from CanonicalJSON")
			}
			if digest != DigestHex(DomainSeparatorRequest, canonical) {
				t.Fatal("digest diverged from DigestHex")
			}
		})
	}
}

func readCopiedGolden(t *testing.T) []byte {
	t.Helper()
	_, file, _, ok := runtime.Caller(0)
	if !ok {
		t.Fatal("runtime.Caller failed")
	}
	raw, err := os.ReadFile(filepath.Join(filepath.Dir(file), "testdata", "golden-v1.json"))
	if err != nil {
		t.Fatal(err)
	}
	return raw
}

func loadGolden(t *testing.T) goldenFile {
	t.Helper()
	var golden goldenFile
	if err := json.Unmarshal(readCopiedGolden(t), &golden); err != nil {
		t.Fatal(err)
	}
	return golden
}

func decodeGoldenPayment(t *testing.T, vector goldenVector) ([]byte, string) {
	t.Helper()
	_, canonical, digest, err := decodePayment(vector.Kind, vector.Input)
	if err != nil {
		t.Fatalf("accepted fixture %s failed: %v", vector.Name, err)
	}
	return canonical, digest
}

func decodePayment(kind string, raw []byte) (any, []byte, string, error) {
	switch kind {
	case "payment_request_v1":
		req, canonical, digest, err := DecodePaymentRequest(raw)
		return req, canonical, digest, err
	case "payment_status_event_v1":
		event, canonical, digest, err := DecodePaymentStatus(raw)
		return event, canonical, digest, err
	default:
		return nil, nil, "", errors.New("unsupported fixture kind")
	}
}

func goldenInvalidBytes(t *testing.T, vector goldenVector) []byte {
	t.Helper()
	switch {
	case vector.HexBytes != "":
		raw, err := hex.DecodeString(vector.HexBytes)
		if err != nil {
			t.Fatal(err)
		}
		return raw
	case vector.RawJSON != "":
		return []byte(vector.RawJSON)
	case len(vector.Input) > 0:
		return vector.Input
	default:
		t.Fatalf("invalid vector %s has no payload", vector.Name)
		return nil
	}
}

func baseRequest() map[string]any {
	return map[string]any{
		"v":             1,
		"request_id":    "00112233445566778899aabbccddeeff",
		"payer_peer_id": "12D3KooWPayer",
		"payee_peer_id": "12D3KooWPayee",
		"asset":         "ZEC",
		"network":       "zec-testnet",
		"amount_atomic": "100000000",
		"receiver":      "u1testreceiver",
		"receiver_kind": "zec-ua-orchard-protocol",
		"memo":          "coffee",
		"nonce":         "ffeeddccbbaa99887766554433221100",
		"created_at":    "2026-08-30T12:00:00Z",
		"expires_at":    "2026-08-30T12:15:00Z",
	}
}

func baseStatus() map[string]any {
	return map[string]any{
		"v":          1,
		"request_id": "00112233445566778899aabbccddeeff",
		"event_id":   "11112222333344445555666677778888",
		"nonce":      "9999aaaabbbbccccddddeeeeffff0000",
		"status":     StatusCancelled,
		"at":         "2026-08-30T12:05:00Z",
		"tx_ref":     "",
	}
}

func mustJSON(t testing.TB, value any) []byte {
	t.Helper()
	raw, err := json.Marshal(value)
	if err != nil {
		t.Fatal(err)
	}
	return raw
}

func replaceRequestJSON(t *testing.T, old, new string) string {
	t.Helper()
	raw := string(mustJSON(t, baseRequest()))
	if !strings.Contains(raw, old) {
		t.Fatalf("request JSON %s does not contain %s", raw, old)
	}
	return strings.Replace(raw, old, new, 1)
}

func replaceStatusJSON(t *testing.T, old, new string) string {
	t.Helper()
	raw := string(mustJSON(t, baseStatus()))
	if !strings.Contains(raw, old) {
		t.Fatalf("status JSON %s does not contain %s", raw, old)
	}
	return strings.Replace(raw, old, new, 1)
}

func withoutKey(value map[string]any, key string) map[string]any {
	clone := make(map[string]any, len(value))
	for k, v := range value {
		if k != key {
			clone[k] = v
		}
	}
	return clone
}

func withField(value map[string]any, key string, field any) map[string]any {
	clone := make(map[string]any, len(value)+1)
	for k, v := range value {
		clone[k] = v
	}
	clone[key] = field
	return clone
}

func codeOf(err error) string {
	if err == nil {
		return ""
	}
	var perr *Error
	if errors.As(err, &perr) {
		return perr.Code
	}
	return ""
}
