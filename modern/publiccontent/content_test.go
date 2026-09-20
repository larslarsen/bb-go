package publiccontent

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"reflect"
	"runtime"
	"strings"
	"testing"
	"unicode/utf8"

	"github.com/ipfs/go-cid"
	mh "github.com/multiformats/go-multihash"
)

const independentRawCID = "bafkreihdwdcefgh4dqkjv67uzcmw7ojee6xedzdetojuzjevtenxquvyku"

type contentVector struct {
	Name      string `json:"name"`
	Input     string `json:"input"`
	Canonical string `json:"canonical"`
	PlainText string `json:"plainText"`
	Error     string `json:"error"`
}

func TestMEDIA001M1PVectors(t *testing.T) {
	raw, err := os.ReadFile("testdata/v1-vectors.json")
	if err != nil {
		t.Fatal(err)
	}
	decoder := json.NewDecoder(bytes.NewReader(raw))
	decoder.DisallowUnknownFields()
	var vectors []contentVector
	if err := decoder.Decode(&vectors); err != nil {
		t.Fatal(err)
	}
	if len(vectors) < 10 {
		t.Fatalf("vector count=%d want at least 10", len(vectors))
	}
	seenSuccess := 0
	seenFailure := 0
	for _, vector := range vectors {
		vector := vector
		t.Run(vector.Name, func(t *testing.T) {
			content, err := Parse([]byte(vector.Input))
			switch vector.Error {
			case "":
				seenSuccess++
				if err != nil {
					t.Fatal(err)
				}
				canonical, err := Marshal(content)
				if err != nil {
					t.Fatal(err)
				}
				if string(canonical) != vector.Canonical {
					t.Fatalf("canonical\n got: %s\nwant: %s", canonical, vector.Canonical)
				}
				plain, err := PlainText(content)
				if err != nil {
					t.Fatal(err)
				}
				if plain != vector.PlainText {
					t.Fatalf("plain text=%q want=%q", plain, vector.PlainText)
				}
				reparsed, err := Parse(canonical)
				if err != nil || !reflect.DeepEqual(reparsed, content) {
					t.Fatalf("canonical reparse=%+v err=%v want=%+v", reparsed, err, content)
				}
			case "invalid":
				seenFailure++
				if !errors.Is(err, ErrInvalidContent) {
					t.Fatalf("error=%v want ErrInvalidContent", err)
				}
			case "unsupported_schema":
				seenFailure++
				if !errors.Is(err, ErrUnsupportedSchema) {
					t.Fatalf("error=%v want ErrUnsupportedSchema", err)
				}
			default:
				t.Fatalf("unknown vector error class %q", vector.Error)
			}
		})
	}
	if seenSuccess == 0 || seenFailure == 0 {
		t.Fatalf("non-vacuous vectors successes=%d failures=%d", seenSuccess, seenFailure)
	}
	if decoded, err := cid.Decode(independentRawCID); err != nil || decoded.Version() != 1 || decoded.Type() != cid.Raw {
		t.Fatalf("independent CID fixture invalid: %v, %v", decoded, err)
	}
}

func TestMEDIA001M1PLimits(t *testing.T) {
	t.Run("input bytes", func(t *testing.T) {
		base := mustMarshalTestJSON(t, validContent())
		for _, size := range []int{65535, 65536} {
			padded := append(append([]byte(nil), base...), bytes.Repeat([]byte(" "), size-len(base))...)
			if len(padded) != size {
				t.Fatalf("fixture size=%d want=%d", len(padded), size)
			}
			if _, err := Parse(padded); err != nil {
				t.Fatalf("size %d rejected: %v", size, err)
			}
		}
		above := append(append([]byte(nil), base...), bytes.Repeat([]byte(" "), 65537-len(base))...)
		assertInvalid(t, func() error { _, err := Parse(above); return err })

		largeOutput := validContent()
		largeOutput.Body[0].Children = make([]InlineNode, 256)
		for index := range largeOutput.Body[0].Children {
			largeOutput.Body[0].Children[index] = InlineNode{Type: "text", Text: "x", Href: "https://example.com/" + strings.Repeat("a", 1900)}
		}
		assertInvalid(t, func() error { _, err := Marshal(largeOutput); return err })

		boundaryOutput := validContent()
		boundaryOutput.Body[0].Children = make([]InlineNode, 256)
		for index := range boundaryOutput.Body[0].Children {
			boundaryOutput.Body[0].Children[index] = InlineNode{Type: "text", Text: "x", Href: "https://e.co/"}
		}
		encoded := mustMarshalContent(t, boundaryOutput)
		remaining := 65536 - len(encoded)
		for index := range boundaryOutput.Body[0].Children {
			available := 2048 - len(boundaryOutput.Body[0].Children[index].Href)
			if available > remaining {
				available = remaining
			}
			boundaryOutput.Body[0].Children[index].Href += strings.Repeat("a", available)
			remaining -= available
		}
		if remaining != 0 {
			t.Fatalf("could not construct exact output boundary; remaining=%d", remaining)
		}
		encoded = mustMarshalContent(t, boundaryOutput)
		if len(encoded) != 65536 {
			t.Fatalf("canonical size=%d want=65536", len(encoded))
		}
		for index := range boundaryOutput.Body[0].Children {
			if len(boundaryOutput.Body[0].Children[index].Href) < 2048 {
				boundaryOutput.Body[0].Children[index].Href += "a"
				break
			}
		}
		assertInvalid(t, func() error { _, err := Marshal(boundaryOutput); return err })
	})

	t.Run("paragraph and inline counts", func(t *testing.T) {
		for _, count := range []int{1, 32} {
			content := validContent()
			content.Body = make([]Paragraph, count)
			for index := range content.Body {
				content.Body[index] = Paragraph{Type: "paragraph", Children: []InlineNode{}}
			}
			content.Body[0].Children = []InlineNode{{Type: "text", Text: "x"}}
			mustMarshalContent(t, content)
		}
		content := validContent()
		content.Body = make([]Paragraph, 33)
		for index := range content.Body {
			content.Body[index] = Paragraph{Type: "paragraph", Children: []InlineNode{}}
		}
		content.Body[0].Children = []InlineNode{{Type: "text", Text: "x"}}
		assertInvalid(t, func() error { _, err := Marshal(content); return err })

		content = validContent()
		content.Body[0].Children = make([]InlineNode, 256)
		for index := range content.Body[0].Children {
			content.Body[0].Children[index] = InlineNode{Type: "text", Text: "x"}
		}
		mustMarshalContent(t, content)
		content.Body[0].Children = append(content.Body[0].Children, InlineNode{Type: "break"})
		assertInvalid(t, func() error { _, err := Marshal(content); return err })
	})

	t.Run("plain scalar count", func(t *testing.T) {
		for _, count := range []int{279, 280} {
			content := validContent()
			content.Body[0].Children[0].Text = strings.Repeat("🙂", count)
			plain, err := PlainText(content)
			if err != nil || utf8.RuneCountInString(plain) != count || len(plain) <= count {
				t.Fatalf("count=%d bytes=%d runes=%d err=%v", count, len(plain), utf8.RuneCountInString(plain), err)
			}
		}
		content := validContent()
		content.Body[0].Children[0].Text = strings.Repeat("x", 281)
		assertInvalid(t, func() error { _, err := PlainText(content); return err })
		content.Body = []Paragraph{
			{Type: "paragraph", Children: []InlineNode{{Type: "text", Text: strings.Repeat("x", 279)}}},
			{Type: "paragraph", Children: []InlineNode{}},
		}
		mustMarshalContent(t, content)
		content.Body[0].Children[0].Text += "x"
		assertInvalid(t, func() error { _, err := Marshal(content); return err })
	})

	t.Run("attachment count and aggregate", func(t *testing.T) {
		content := contentWithAttachments(8, 1, "image/jpeg")
		mustMarshalContent(t, content)
		content = contentWithAttachments(9, 1, "image/jpeg")
		assertInvalid(t, func() error { _, err := Marshal(content); return err })

		content = contentWithAttachments(2, 100<<20, "video/mp4")
		mustMarshalContent(t, content)
		content = contentWithAttachments(3, 1, "video/mp4")
		content.Attachments[0].ByteLength = 100 << 20
		content.Attachments[1].ByteLength = 100 << 20
		assertInvalid(t, func() error { _, err := Marshal(content); return err })
	})

	t.Run("file dimensions duration and filename", func(t *testing.T) {
		image := contentWithAttachments(1, 25<<20, "image/jpeg")
		mustMarshalContent(t, image)
		image.Attachments[0].ByteLength++
		assertInvalid(t, func() error { _, err := Marshal(image); return err })
		image.Attachments[0].ByteLength = 0
		assertInvalid(t, func() error { _, err := Marshal(image); return err })

		video := contentWithAttachments(1, 100<<20, "video/mp4")
		mustMarshalContent(t, video)
		video.Attachments[0].ByteLength++
		assertInvalid(t, func() error { _, err := Marshal(video); return err })

		image = contentWithAttachments(1, 1, "image/png")
		image.Attachments[0].Width = 8000
		image.Attachments[0].Height = 5000
		mustMarshalContent(t, image)
		image.Attachments[0].Height = 5001
		assertInvalid(t, func() error { _, err := Marshal(image); return err })
		for _, width := range []uint64{0, 8193} {
			image = contentWithAttachments(1, 1, "image/png")
			image.Attachments[0].Width = width
			assertInvalid(t, func() error { _, err := Marshal(image); return err })
		}

		for _, test := range []struct {
			media string
			value uint64
			valid bool
		}{
			{media: "image/gif", value: 1, valid: true},
			{media: "image/gif", value: 60000, valid: true},
			{media: "image/gif", value: 0},
			{media: "image/gif", value: 60001},
			{media: "video/webm", value: 1, valid: true},
			{media: "video/webm", value: 600000, valid: true},
			{media: "video/webm", value: 0},
			{media: "video/webm", value: 600001},
		} {
			content := contentWithAttachments(1, 1, test.media)
			content.Attachments[0].DurationMS = uint64Pointer(test.value)
			if test.valid {
				mustMarshalContent(t, content)
			} else {
				assertInvalid(t, func() error { _, err := Marshal(content); return err })
			}
		}
		image = contentWithAttachments(1, 1, "image/webp")
		image.Attachments[0].DurationMS = uint64Pointer(1)
		assertInvalid(t, func() error { _, err := Marshal(image); return err })

		image = contentWithAttachments(1, 1, "image/jpeg")
		image.Attachments[0].Filename = strings.Repeat("🙂", 255)
		mustMarshalContent(t, image)
		image.Attachments[0].Filename += "🙂"
		assertInvalid(t, func() error { _, err := Marshal(image); return err })
	})

	t.Run("safe links", func(t *testing.T) {
		prefix := "https://example.com/"
		content := validContent()
		content.Body[0].Children[0].Href = prefix + strings.Repeat("a", 2048-len(prefix))
		mustMarshalContent(t, content)
		content.Body[0].Children[0].Href += "a"
		assertInvalid(t, func() error { _, err := Marshal(content); return err })
		for _, link := range []string{
			"HTTP://example.com", "//example.com", "javascript:alert(1)", "data:text/plain,x",
			"file:///tmp/x", "blob:https://example.com/x", "http://", "http://user@example.com",
			"https://example.com/a b", "https://example.com/a\\b", "https://example.com/%00",
			"https://example.com/%1f", "https://example.com/%7F", "https://example.com/%5c",
			"https://example.com/%zz", "https://example.com/\n",
		} {
			content := validContent()
			content.Body[0].Children[0].Href = link
			assertInvalid(t, func() error { _, err := Marshal(content); return err })
		}
	})
}

func TestMEDIA001M1PTypedValidationParity(t *testing.T) {
	for _, size := range []int{65535, 65536} {
		content, expected := independentlySizedContent(t, size)
		encoded, err := Marshal(content)
		if err != nil || !bytes.Equal(encoded, expected) {
			t.Fatalf("size %d Marshal length=%d err=%v", size, len(encoded), err)
		}
		plain, err := PlainText(content)
		if err != nil || plain != strings.Repeat("x", 256) {
			t.Fatalf("size %d PlainText length=%d err=%v", size, len(plain), err)
		}
	}

	oversized, expected := independentlySizedContent(t, 65537)
	if len(expected) != 65537 {
		t.Fatalf("independent oversized canonical length=%d", len(expected))
	}
	assertTypedEntryPointsInvalid(t, oversized)

	longLinks := validContent()
	longLinks.Body[0].Children = make([]InlineNode, 256)
	for index := range longLinks.Body[0].Children {
		longLinks.Body[0].Children[index] = InlineNode{Type: "text", Text: "x", Href: "https://example.com/" + strings.Repeat("a", 1900)}
	}
	assertTypedEntryPointsInvalid(t, longLinks)

	escapedContent := validContent()
	escapedContent.Body[0].Children = make([]InlineNode, 256)
	for index := range escapedContent.Body[0].Children {
		escapedContent.Body[0].Children[index] = InlineNode{
			Type: "text", Text: "x", Href: "https://e.co/?" + strings.Repeat("&", 100),
		}
	}
	raw := independentASCIITextDocument(t, escapedContent)
	expectedEscapedSize := len(raw) + 5*100*256
	if len(raw) >= 65536 || expectedEscapedSize <= 65536 {
		t.Fatalf("escaped fixture raw=%d canonical=%d", len(raw), expectedEscapedSize)
	}
	assertInvalid(t, func() error { _, err := Parse(raw); return err })
	assertTypedEntryPointsInvalid(t, escapedContent)

	emptySchemaWire := []byte(`{"schema":"","body":[{"type":"paragraph","children":[{"type":"text","text":"x","marks":[],"href":""}]}],"attachments":[]}`)
	assertInvalid(t, func() error { _, err := Parse(emptySchemaWire); return err })
	emptySchema := validContent()
	emptySchema.Schema = ""
	assertTypedEntryPointsInvalid(t, emptySchema)

	invalidSchema := validContent()
	invalidSchema.Schema = string([]byte{0xff})
	assertTypedEntryPointsInvalid(t, invalidSchema)
	invalidSchemaWire := bytes.Replace(mustMarshalTestJSON(t, validContent()), []byte(`bitbook.public-content/1`), []byte{0xff}, 1)
	assertInvalid(t, func() error { _, err := Parse(invalidSchemaWire); return err })

	otherSchemaWire := []byte(`{"schema":"bitbook.public-content/2","body":[{"type":"paragraph","children":[{"type":"text","text":"x","marks":[],"href":""}]}],"attachments":[]}`)
	if _, err := Parse(otherSchemaWire); !errors.Is(err, ErrUnsupportedSchema) {
		t.Fatalf("wire other schema error=%v", err)
	}
	otherSchema := validContent()
	otherSchema.Schema = "bitbook.public-content/2"
	for name, call := range map[string]func() error{
		"Marshal":   func() error { _, err := Marshal(otherSchema); return err },
		"PlainText": func() error { _, err := PlainText(otherSchema); return err },
	} {
		if err := call(); !errors.Is(err, ErrUnsupportedSchema) {
			t.Fatalf("%s other schema error=%v", name, err)
		}
	}
}

func TestMEDIA001M1PTypedRejectionBounds(t *testing.T) {
	manyChildren := validContent()
	manyChildren.Body[0].Children = make([]InlineNode, 4096)
	for index := range manyChildren.Body[0].Children {
		manyChildren.Body[0].Children[index] = InlineNode{Type: "text", Text: "x"}
	}
	hugeText := validContent()
	hugeText.Body[0].Children[0].Text = strings.Repeat("x", 1<<20)

	for _, fixture := range []struct {
		name    string
		content Content
	}{
		{name: "4096 children", content: manyChildren},
		{name: "1 MiB text", content: hugeText},
	} {
		for _, entry := range []struct {
			name string
			call func(Content) error
		}{
			{name: "Marshal", call: func(content Content) error { _, err := Marshal(content); return err }},
			{name: "PlainText", call: func(content Content) error { _, err := PlainText(content); return err }},
		} {
			t.Run(fixture.name+"/"+entry.name, func(t *testing.T) {
				allocated := minimumRejectedAllocation(t, func() error { return entry.call(fixture.content) })
				if allocated > 64<<10 {
					t.Fatalf("rejection allocated %d bytes; ceiling=%d", allocated, 64<<10)
				}
			})
		}
	}

	normalChildren := validContent()
	normalChildren.Body[0].Children = make([]InlineNode, 256)
	for index := range normalChildren.Body[0].Children {
		normalChildren.Body[0].Children[index] = InlineNode{Type: "text", Text: "x"}
	}
	normalScalars := validContent()
	normalScalars.Body[0].Children[0].Text = strings.Repeat("🙂", 280)
	for _, content := range []Content{normalChildren, normalScalars} {
		mustMarshalContent(t, content)
		if _, err := PlainText(content); err != nil {
			t.Fatalf("positive typed control rejected: %v", err)
		}
	}
}

func TestMEDIA001M1PStrictJSON(t *testing.T) {
	valid := string(mustMarshalTestJSON(t, validContent()))
	cases := []struct {
		name string
		raw  []byte
	}{
		{name: "empty", raw: nil},
		{name: "malformed", raw: []byte(`{"schema":`)},
		{name: "trailing value", raw: []byte(valid + ` {}`)},
		{name: "invalid utf8", raw: append([]byte(`{"schema":"`), 0xff)},
		{name: "unpaired high surrogate", raw: []byte(strings.Replace(valid, "x", `\ud800`, 1))},
		{name: "unpaired low surrogate", raw: []byte(strings.Replace(valid, "x", `\udc00`, 1))},
		{name: "missing schema", raw: []byte(`{"body":[{"type":"paragraph","children":[]}],"attachments":[]}`)},
		{name: "null schema", raw: []byte(`{"schema":null,"body":[{"type":"paragraph","children":[]}],"attachments":[]}`)},
		{name: "null body", raw: []byte(`{"schema":"bitbook.public-content/1","body":null,"attachments":[]}`)},
		{name: "null attachments", raw: []byte(`{"schema":"bitbook.public-content/1","body":[{"type":"paragraph","children":[]}],"attachments":null}`)},
		{name: "wrong body type", raw: []byte(`{"schema":"bitbook.public-content/1","body":{},"attachments":[]}`)},
		{name: "unknown top", raw: []byte(`{"schema":"bitbook.public-content/1","body":[{"type":"paragraph","children":[]}],"attachments":[],"privateKey":"x"}`)},
		{name: "unknown paragraph", raw: []byte(`{"schema":"bitbook.public-content/1","body":[{"type":"paragraph","children":[],"html":"x"}],"attachments":[]}`)},
		{name: "unknown node", raw: []byte(`{"schema":"bitbook.public-content/1","body":[{"type":"paragraph","children":[{"type":"break","provider":"x"}]}],"attachments":[]}`)},
		{name: "wrong paragraph type", raw: []byte(`{"schema":"bitbook.public-content/1","body":[{"type":"heading","children":[]}],"attachments":[]}`)},
		{name: "null children", raw: []byte(`{"schema":"bitbook.public-content/1","body":[{"type":"paragraph","children":null}],"attachments":[]}`)},
		{name: "missing text marks", raw: []byte(`{"schema":"bitbook.public-content/1","body":[{"type":"paragraph","children":[{"type":"text","text":"x","href":""}]}],"attachments":[]}`)},
		{name: "null text", raw: []byte(`{"schema":"bitbook.public-content/1","body":[{"type":"paragraph","children":[{"type":"text","text":null,"marks":[],"href":""}]}],"attachments":[]}`)},
		{name: "null marks", raw: []byte(`{"schema":"bitbook.public-content/1","body":[{"type":"paragraph","children":[{"type":"text","text":"x","marks":null,"href":""}]}],"attachments":[]}`)},
		{name: "break extra field", raw: []byte(`{"schema":"bitbook.public-content/1","body":[{"type":"paragraph","children":[{"type":"break","text":"x"}]}],"attachments":[]}`)},
		{name: "attachment extra field", raw: []byte(`{"schema":"bitbook.public-content/1","body":[{"type":"paragraph","children":[{"type":"attachment","attachmentId":"01010101010101010101010101010101","href":""}]}],"attachments":[]}`)},
		{name: "unknown node type", raw: []byte(`{"schema":"bitbook.public-content/1","body":[{"type":"paragraph","children":[{"type":"gif","provider":"klipy"}]}],"attachments":[]}`)},
		{name: "nested recipient", raw: []byte(`{"schema":"bitbook.public-content/1","body":[{"type":"paragraph","children":[{"type":"text","text":"x","marks":[],"href":"","recipient":"p"}]}],"attachments":[]}`)},
		{name: "excess depth", raw: []byte(`[[[[[[[[[]]]]]]]]]`)},
	}
	for _, test := range cases {
		t.Run(test.name, func(t *testing.T) {
			assertInvalid(t, func() error { _, err := Parse(test.raw); return err })
		})
	}

	wrongSchema := []byte(`{"schema":"bitbook.public-content/2","body":[{"type":"paragraph","children":[{"type":"text","text":"x","marks":[],"href":""}]}],"attachments":[]}`)
	if _, err := Parse(wrongSchema); !errors.Is(err, ErrUnsupportedSchema) {
		t.Fatalf("wrong schema error=%v", err)
	}
}

func TestMEDIA001M1PPreflightGuards(t *testing.T) {
	base := []byte(`{"schema":"bitbook.public-content/1","body":[{"type":"paragraph","children":[{"type":"text","text":"hello","marks":[],"href":""}]}],"attachments":[]}`)
	invalidUTF8 := replaceTextValueBytes(t, base, []byte{'h', 0xff})
	loneHigh := replaceTextValueBytes(t, base, []byte(`\ud800`))
	loneLow := replaceTextValueBytes(t, base, []byte(`\udc00`))
	for name, raw := range map[string][]byte{
		"invalid UTF-8": invalidUTF8,
		"lone high":     loneHigh,
		"lone low":      loneLow,
	} {
		t.Run(name, func(t *testing.T) {
			assertInvalid(t, func() error { _, err := Parse(raw); return err })
		})
	}

	pair := replaceTextValueBytes(t, base, []byte(`\ud83d\ude42`))
	parsed, err := Parse(pair)
	if err != nil {
		t.Fatalf("escaped surrogate pair rejected: %v", err)
	}
	if plain, err := PlainText(parsed); err != nil || plain != "🙂" {
		t.Fatalf("escaped pair plain=%q err=%v", plain, err)
	}
	replacement := replaceTextValueBytes(t, base, []byte("�"))
	parsed, err = Parse(replacement)
	if err != nil {
		t.Fatalf("literal replacement character rejected: %v", err)
	}
	if plain, err := PlainText(parsed); err != nil || plain != "�" {
		t.Fatalf("replacement plain=%q err=%v", plain, err)
	}

	for _, depth := range []int{7, 8} {
		if err := preflightJSON(nestedJSON(depth)); err != nil {
			t.Fatalf("depth %d rejected at preflight: %v", depth, err)
		}
	}
	if err := preflightJSON(nestedJSON(9)); !errors.Is(err, ErrInvalidContent) {
		t.Fatalf("depth 9 preflight error=%v", err)
	}
	for _, depth := range []int{7, 8, 9} {
		assertInvalid(t, func() error { _, err := Parse(nestedJSON(depth)); return err })
	}
}

func TestMEDIA001M1PDescriptorWireRules(t *testing.T) {
	id := testAttachmentID(1)
	valid := fmt.Sprintf(`{"schema":"bitbook.public-content/1","body":[{"type":"paragraph","children":[{"type":"attachment","attachmentId":%q}]}],"attachments":[{"attachmentId":%q,"cid":%q,"mediaType":"image/jpeg","byteLength":1,"width":1,"height":1,"durationMs":null,"filename":""}]}`, id, id, independentRawCID)
	if _, err := Parse([]byte(valid)); err != nil {
		t.Fatalf("valid descriptor wire rejected: %v", err)
	}

	cases := []struct {
		name string
		raw  string
	}{
		{name: "negative integer", raw: strings.Replace(valid, `"byteLength":1`, `"byteLength":-1`, 1)},
		{name: "fraction integer", raw: strings.Replace(valid, `"byteLength":1`, `"byteLength":1.0`, 1)},
		{name: "exponent integer", raw: strings.Replace(valid, `"byteLength":1`, `"byteLength":1e0`, 1)},
		{name: "leading zero integer", raw: strings.Replace(valid, `"byteLength":1`, `"byteLength":01`, 1)},
		{name: "string integer", raw: strings.Replace(valid, `"byteLength":1`, `"byteLength":"1"`, 1)},
		{name: "null integer", raw: strings.Replace(valid, `"byteLength":1`, `"byteLength":null`, 1)},
		{name: "integer overflow", raw: strings.Replace(valid, `"byteLength":1`, `"byteLength":18446744073709551616`, 1)},
		{name: "missing field", raw: strings.Replace(valid, `,"filename":""`, ``, 1)},
		{name: "unknown field", raw: strings.Replace(valid, `,"filename":""`, `,"filename":"","privateKey":"x"`, 1)},
		{name: "null descriptor", raw: strings.Replace(valid, `{"attachmentId":`+fmt.Sprintf("%q", id)+`,"cid"`, `null,{"attachmentId":`+fmt.Sprintf("%q", id)+`,"cid"`, 1)},
		{name: "null string", raw: strings.Replace(valid, `"filename":""`, `"filename":null`, 1)},
		{name: "static duration integer", raw: strings.Replace(valid, `"durationMs":null`, `"durationMs":1`, 1)},
		{name: "gif duration null", raw: strings.Replace(strings.Replace(valid, `"mediaType":"image/jpeg"`, `"mediaType":"image/gif"`, 1), `"durationMs":null`, `"durationMs":null`, 1)},
	}
	for _, test := range cases {
		t.Run(test.name, func(t *testing.T) {
			assertInvalid(t, func() error { _, err := Parse([]byte(test.raw)); return err })
		})
	}
}

func TestMEDIA001M1PDuplicateKeys(t *testing.T) {
	cases := [][]byte{
		[]byte(`{"schema":"bitbook.public-content/1","schema":"bitbook.public-content/2","body":[{"type":"paragraph","children":[]}],"attachments":[]}`),
		[]byte(`{"schema":"bitbook.public-content/1","\u0073chema":"bitbook.public-content/1","body":[{"type":"paragraph","children":[]}],"attachments":[]}`),
		[]byte(`{"schema":"bitbook.public-content/1","body":[{"type":"paragraph","children":[{"type":"text","text":"safe","text":"evil","marks":[],"href":""}]}],"attachments":[]}`),
		[]byte(`{"schema":"bitbook.public-content/1","body":[{"type":"paragraph","children":[],"children":[]}],"attachments":[]}`),
	}
	for index, raw := range cases {
		if _, err := Parse(raw); !errors.Is(err, ErrInvalidContent) {
			t.Fatalf("case %d error=%v want ErrInvalidContent", index, err)
		}
	}
}

func TestMEDIA001M1PAttachmentValidation(t *testing.T) {
	t.Run("references", func(t *testing.T) {
		content := contentWithAttachments(1, 1, "image/jpeg")
		content.Body[0].Children = append(content.Body[0].Children, content.Body[0].Children[0])
		assertInvalid(t, func() error { _, err := Marshal(content); return err })
		content = contentWithAttachments(1, 1, "image/jpeg")
		content.Body[0].Children[0].AttachmentID = testAttachmentID(2)
		assertInvalid(t, func() error { _, err := Marshal(content); return err })
		content = contentWithAttachments(2, 1, "image/jpeg")
		content.Attachments[1].AttachmentID = content.Attachments[0].AttachmentID
		assertInvalid(t, func() error { _, err := Marshal(content); return err })
		content = contentWithAttachments(2, 1, "image/jpeg")
		content.Body[0].Children = content.Body[0].Children[:1]
		assertInvalid(t, func() error { _, err := Marshal(content); return err })
		for _, id := range []string{"", strings.Repeat("0", 32), strings.Repeat("A", 32), "01"} {
			content = contentWithAttachments(1, 1, "image/jpeg")
			content.Attachments[0].AttachmentID = id
			content.Body[0].Children[0].AttachmentID = id
			assertInvalid(t, func() error { _, err := Marshal(content); return err })
		}
	})

	t.Run("CID profile", func(t *testing.T) {
		validDagPB := makeCID(t, cid.DagProtobuf, mh.SHA2_256, []byte("dag-pb"))
		content := contentWithAttachments(1, 1, "image/jpeg")
		content.Attachments[0].CID = validDagPB
		mustMarshalContent(t, content)
		invalid := []string{
			"QmYwAPJzv5CZsnAzt8auVZRnG8Qh9N7ZzH9fV7M2P7h5aK",
			strings.ToUpper(independentRawCID),
			"https://example.com/" + independentRawCID,
			independentRawCID + "/path",
			makeCID(t, cid.Raw, mh.SHA2_512, []byte("sha512")),
			makeCID(t, 0x71, mh.SHA2_256, []byte("wrong codec")),
			makeCID(t, cid.Raw, mh.IDENTITY, []byte("identity")),
		}
		for _, value := range invalid {
			content := contentWithAttachments(1, 1, "image/jpeg")
			content.Attachments[0].CID = value
			assertInvalid(t, func() error { _, err := Marshal(content); return err })
		}
	})

	t.Run("media and filename", func(t *testing.T) {
		for _, mediaType := range []string{"", "image/svg+xml", "text/html", "video/quicktime", "IMAGE/JPEG"} {
			content := contentWithAttachments(1, 1, "image/jpeg")
			content.Attachments[0].MediaType = mediaType
			assertInvalid(t, func() error { _, err := Marshal(content); return err })
		}
		for _, filename := range []string{"a/b", `a\b`, "a\x00b", "a\x7fb", "a\nb"} {
			content := contentWithAttachments(1, 1, "image/jpeg")
			content.Attachments[0].Filename = filename
			assertInvalid(t, func() error { _, err := Marshal(content); return err })
		}
	})

	t.Run("repeated CID and descriptor order", func(t *testing.T) {
		content := contentWithAttachments(2, 1, "image/jpeg")
		content.Attachments[0], content.Attachments[1] = content.Attachments[1], content.Attachments[0]
		encoded := mustMarshalContent(t, content)
		first := bytes.LastIndex(encoded, []byte(`"attachmentId":"`+testAttachmentID(2)+`"`))
		second := bytes.LastIndex(encoded, []byte(`"attachmentId":"`+testAttachmentID(1)+`"`))
		if first < 0 || second < 0 || first >= second {
			t.Fatalf("descriptor order not preserved: %s", encoded)
		}
	})
}

func TestMEDIA001M1PCanonicalOwnershipAndTypedValidation(t *testing.T) {
	content := validContent()
	content.Body[0].Children[0].Marks = []string{"code", "bold"}
	content.Body = append(content.Body, Paragraph{Type: "paragraph", Children: nil})
	before := append([]string(nil), content.Body[0].Children[0].Marks...)
	encoded, err := Marshal(content)
	if err != nil {
		t.Fatal(err)
	}
	if !reflect.DeepEqual(content.Body[0].Children[0].Marks, before) || content.Body[1].Children != nil {
		t.Fatal("Marshal mutated caller-owned slices")
	}
	if !bytes.Contains(encoded, []byte(`"marks":["bold","code"]`)) || !bytes.Contains(encoded, []byte(`"children":[]`)) {
		t.Fatalf("canonical normalization missing: %s", encoded)
	}
	parsed, err := Parse(encoded)
	if err != nil {
		t.Fatal(err)
	}
	again, err := Marshal(parsed)
	if err != nil || !bytes.Equal(again, encoded) {
		t.Fatalf("idempotent marshal=%s err=%v want=%s", again, err, encoded)
	}
	input := append([]byte(nil), encoded...)
	parsed, err = Parse(input)
	if err != nil {
		t.Fatal(err)
	}
	clear(input)
	plain, err := PlainText(parsed)
	if err != nil || plain != "x\n" {
		t.Fatalf("input alias plain=%q err=%v", plain, err)
	}
	clear(encoded)
	third, err := Marshal(parsed)
	if err != nil || bytes.Count(third, []byte("bitbook.public-content/1")) != 1 {
		t.Fatalf("output alias changed content: %s, %v", third, err)
	}

	unicodeContent := validContent()
	unicodeContent.Body[0].Children[0].Text = "x\u2028y\u2029z"
	unicodeJSON := mustMarshalContent(t, unicodeContent)
	if !bytes.Contains(unicodeJSON, []byte(`x\u2028y\u2029z`)) {
		t.Fatalf("U+2028/U+2029 not escaped: %s", unicodeJSON)
	}

	invalidTyped := []Content{
		{},
		{Schema: "bitbook.public-content/2", Body: validContent().Body, Attachments: []AttachmentDescriptor{}},
		{Schema: "bitbook.public-content/1", Body: []Paragraph{}, Attachments: []AttachmentDescriptor{}},
		{Schema: "bitbook.public-content/1", Body: []Paragraph{{Type: "heading", Children: []InlineNode{{Type: "text", Text: "x"}}}}, Attachments: []AttachmentDescriptor{}},
		{Schema: "bitbook.public-content/1", Body: []Paragraph{{Type: "paragraph", Children: []InlineNode{{Type: "text", Text: ""}}}}, Attachments: []AttachmentDescriptor{}},
		{Schema: "bitbook.public-content/1", Body: []Paragraph{{Type: "paragraph", Children: []InlineNode{{Type: "text", Text: "x\n"}}}}, Attachments: []AttachmentDescriptor{}},
		{Schema: "bitbook.public-content/1", Body: []Paragraph{{Type: "paragraph", Children: []InlineNode{{Type: "text", Text: "x\r"}}}}, Attachments: []AttachmentDescriptor{}},
		{Schema: "bitbook.public-content/1", Body: []Paragraph{{Type: "paragraph", Children: []InlineNode{{Type: "text", Text: "x\x00"}}}}, Attachments: []AttachmentDescriptor{}},
		{Schema: "bitbook.public-content/1", Body: []Paragraph{{Type: "paragraph", Children: []InlineNode{{Type: "text", Text: "x", Marks: []string{"underline"}}}}}, Attachments: []AttachmentDescriptor{}},
		{Schema: "bitbook.public-content/1", Body: []Paragraph{{Type: "paragraph", Children: []InlineNode{{Type: "text", Text: "x", Marks: []string{"bold", "bold"}}}}}, Attachments: []AttachmentDescriptor{}},
		{Schema: "bitbook.public-content/1", Body: []Paragraph{{Type: "paragraph", Children: []InlineNode{{Type: "break", Text: "x"}}}}, Attachments: []AttachmentDescriptor{}},
		{Schema: "bitbook.public-content/1", Body: []Paragraph{{Type: "paragraph", Children: []InlineNode{{Type: "attachment", AttachmentID: testAttachmentID(1), Marks: []string{}}}}}, Attachments: []AttachmentDescriptor{}},
		{Schema: "bitbook.public-content/1", Body: []Paragraph{{Type: "paragraph", Children: []InlineNode{}}}, Attachments: []AttachmentDescriptor{}},
	}
	for index, invalid := range invalidTyped {
		if _, err := Marshal(invalid); !errors.Is(err, ErrInvalidContent) && !errors.Is(err, ErrUnsupportedSchema) {
			t.Fatalf("typed case %d marshal error=%v", index, err)
		}
		if _, err := PlainText(invalid); !errors.Is(err, ErrInvalidContent) && !errors.Is(err, ErrUnsupportedSchema) {
			t.Fatalf("typed case %d plain error=%v", index, err)
		}
	}
	tab := validContent()
	tab.Body[0].Children[0].Text = "x\ty"
	if plain, err := PlainText(tab); err != nil || plain != "x\ty" {
		t.Fatalf("TAB text plain=%q err=%v", plain, err)
	}
}

func validContent() Content {
	return Content{
		Schema: "bitbook.public-content/1",
		Body: []Paragraph{{
			Type: "paragraph",
			Children: []InlineNode{{
				Type:  "text",
				Text:  "x",
				Marks: []string{},
				Href:  "",
			}},
		}},
		Attachments: []AttachmentDescriptor{},
	}
}

func contentWithAttachments(count int, byteLength uint64, mediaType string) Content {
	content := Content{
		Schema:      "bitbook.public-content/1",
		Body:        []Paragraph{{Type: "paragraph", Children: make([]InlineNode, count)}},
		Attachments: make([]AttachmentDescriptor, count),
	}
	for index := 0; index < count; index++ {
		id := testAttachmentID(index + 1)
		content.Body[0].Children[index] = InlineNode{Type: "attachment", AttachmentID: id}
		descriptor := AttachmentDescriptor{
			AttachmentID: id,
			CID:          independentRawCID,
			MediaType:    mediaType,
			ByteLength:   byteLength,
			Width:        1,
			Height:       1,
			Filename:     "",
		}
		if mediaType == "image/gif" {
			descriptor.DurationMS = uint64Pointer(1)
		} else if strings.HasPrefix(mediaType, "video/") {
			descriptor.DurationMS = uint64Pointer(1)
		}
		content.Attachments[index] = descriptor
	}
	return content
}

func testAttachmentID(value int) string { return fmt.Sprintf("%032x", value) }

func uint64Pointer(value uint64) *uint64 { return &value }

func mustMarshalContent(t testing.TB, content Content) []byte {
	t.Helper()
	encoded, err := Marshal(content)
	if err != nil {
		t.Fatal(err)
	}
	return encoded
}

func mustMarshalTestJSON(t testing.TB, content Content) []byte {
	t.Helper()
	encoded, err := Marshal(content)
	if err != nil {
		t.Fatal(err)
	}
	return encoded
}

func assertInvalid(t testing.TB, call func() error) {
	t.Helper()
	if err := call(); !errors.Is(err, ErrInvalidContent) {
		t.Fatalf("error=%v want ErrInvalidContent", err)
	}
}

func makeCID(t testing.TB, codec uint64, hashCode uint64, data []byte) string {
	t.Helper()
	digest, err := mh.Sum(data, hashCode, -1)
	if err != nil {
		t.Fatal(err)
	}
	return cid.NewCidV1(codec, digest).String()
}

func independentlySizedContent(t testing.TB, target int) (Content, []byte) {
	t.Helper()
	content := validContent()
	content.Body[0].Children = make([]InlineNode, 256)
	for index := range content.Body[0].Children {
		content.Body[0].Children[index] = InlineNode{Type: "text", Text: "x", Href: "https://e.co/"}
	}
	base := independentASCIITextDocument(t, content)
	remaining := target - len(base)
	if remaining < 0 {
		t.Fatalf("target %d below independent base %d", target, len(base))
	}
	for index := range content.Body[0].Children {
		available := 2048 - len(content.Body[0].Children[index].Href)
		if available > remaining {
			available = remaining
		}
		content.Body[0].Children[index].Href += strings.Repeat("a", available)
		remaining -= available
	}
	if remaining != 0 {
		t.Fatalf("target %d exceeds independent fixture capacity by %d", target, remaining)
	}
	expected := independentASCIITextDocument(t, content)
	if len(expected) != target {
		t.Fatalf("independent canonical length=%d want=%d", len(expected), target)
	}
	return content, expected
}

func independentASCIITextDocument(t testing.TB, content Content) []byte {
	t.Helper()
	if len(content.Body) != 1 || content.Body[0].Type != "paragraph" || len(content.Attachments) != 0 {
		t.Fatal("independent encoder only supports one text-only paragraph")
	}
	var result strings.Builder
	result.WriteString(`{"schema":"bitbook.public-content/1","body":[{"type":"paragraph","children":[`)
	for index, node := range content.Body[0].Children {
		if node.Type != "text" || node.Text != "x" || len(node.Marks) != 0 || strings.ContainsAny(node.Href, `"\<>`) {
			t.Fatalf("unsupported independent node %d: %+v", index, node)
		}
		if index > 0 {
			result.WriteByte(',')
		}
		result.WriteString(`{"type":"text","text":"x","marks":[],"href":"`)
		result.WriteString(node.Href)
		result.WriteString(`"}`)
	}
	result.WriteString(`]}],"attachments":[]}`)
	return []byte(result.String())
}

func assertTypedEntryPointsInvalid(t testing.TB, content Content) {
	t.Helper()
	assertInvalid(t, func() error { _, err := Marshal(content); return err })
	assertInvalid(t, func() error { _, err := PlainText(content); return err })
}

func minimumRejectedAllocation(t testing.TB, call func() error) uint64 {
	t.Helper()
	minimum := ^uint64(0)
	for range 3 {
		runtime.GC()
		var before, after runtime.MemStats
		runtime.ReadMemStats(&before)
		err := call()
		runtime.ReadMemStats(&after)
		if !errors.Is(err, ErrInvalidContent) {
			t.Fatalf("rejection error=%v want ErrInvalidContent", err)
		}
		if allocated := after.TotalAlloc - before.TotalAlloc; allocated < minimum {
			minimum = allocated
		}
	}
	return minimum
}

func replaceTextValueBytes(t testing.TB, raw, replacement []byte) []byte {
	t.Helper()
	needle := []byte(`"text":"hello"`)
	if bytes.Count(raw, needle) != 1 {
		t.Fatalf("text-value occurrence count=%d", bytes.Count(raw, needle))
	}
	value := append([]byte(`"text":"`), replacement...)
	value = append(value, '"')
	mutated := bytes.Replace(raw, needle, value, 1)
	if bytes.Contains(mutated, needle) {
		t.Fatal("text-value mutation did not occur")
	}
	return mutated
}

func nestedJSON(depth int) []byte {
	return []byte(strings.Repeat("[", depth) + "0" + strings.Repeat("]", depth))
}
