package publiccontent

import (
	"bytes"
	"reflect"
	"testing"
	"unicode/utf8"
)

func FuzzMEDIA001M1PContent(f *testing.F) {
	valid := []byte(`{"schema":"bitbook.public-content/1","body":[{"type":"paragraph","children":[{"type":"text","text":"hello","marks":[],"href":""}]}],"attachments":[]}`)
	validAttachment := []byte(`{"schema":"bitbook.public-content/1","body":[{"type":"paragraph","children":[{"type":"attachment","attachmentId":"01010101010101010101010101010101"}]}],"attachments":[{"attachmentId":"01010101010101010101010101010101","cid":"bafkreihdwdcefgh4dqkjv67uzcmw7ojee6xedzdetojuzjevtenxquvyku","mediaType":"image/jpeg","byteLength":1,"width":1,"height":1,"durationMs":null,"filename":"x.jpg"}]}`)
	f.Add(valid)
	f.Add([]byte(`{"attachments":[],"body":[{"children":[{"href":"","marks":["code","bold"],"text":"x","type":"text"}],"type":"paragraph"}],"schema":"bitbook.public-content/1"}`))
	f.Add([]byte(`{"schema":"bitbook.public-content/2","body":[{"type":"paragraph","children":[]}],"attachments":[]}`))
	f.Add([]byte(`{"schema":"bitbook.public-content/1","schema":"bitbook.public-content/2","body":[],"attachments":[]}`))
	f.Add([]byte(`{"schema":"bitbook.public-content/1","body":[{"type":"paragraph","children":[{"type":"text","text":"x","marks":null,"href":""}]}],"attachments":[]}`))
	f.Add(validAttachment)
	f.Add([]byte{0xff, 0xfe, 0xfd})
	f.Fuzz(func(t *testing.T, raw []byte) {
		if len(raw) > 70000 {
			t.Skip()
		}
		content, err := Parse(raw)
		if (bytes.Equal(raw, valid) || bytes.Equal(raw, validAttachment)) && err != nil {
			t.Fatalf("valid seed rejected: %v", err)
		}
		if err != nil {
			if len(err.Error()) > 128 {
				t.Fatalf("unbounded error length=%d", len(err.Error()))
			}
			return
		}

		if content.Schema != "bitbook.public-content/1" || len(content.Body) < 1 || len(content.Body) > 32 || len(content.Attachments) > 8 {
			t.Fatalf("accepted invalid top-level counts: %+v", content)
		}
		children := 0
		references := make(map[string]int)
		for _, paragraph := range content.Body {
			if paragraph.Type != "paragraph" {
				t.Fatalf("accepted paragraph type %q", paragraph.Type)
			}
			children += len(paragraph.Children)
			for _, node := range paragraph.Children {
				if node.Type == "attachment" {
					references[node.AttachmentID]++
				}
			}
		}
		if children > 256 {
			t.Fatalf("accepted %d inline children", children)
		}
		seenDescriptors := make(map[string]struct{})
		for _, descriptor := range content.Attachments {
			if references[descriptor.AttachmentID] != 1 {
				t.Fatalf("descriptor reference count=%d for %q", references[descriptor.AttachmentID], descriptor.AttachmentID)
			}
			if _, exists := seenDescriptors[descriptor.AttachmentID]; exists {
				t.Fatalf("duplicate descriptor %q", descriptor.AttachmentID)
			}
			seenDescriptors[descriptor.AttachmentID] = struct{}{}
		}
		for id, count := range references {
			if count != 1 {
				t.Fatalf("attachment node count=%d for %q", count, id)
			}
			if _, exists := seenDescriptors[id]; !exists {
				t.Fatalf("attachment node lacks descriptor %q", id)
			}
		}

		plain, err := PlainText(content)
		if err != nil || !utf8.ValidString(plain) || utf8.RuneCountInString(plain) > 280 {
			t.Fatalf("accepted plain text invalid: runes=%d err=%v", utf8.RuneCountInString(plain), err)
		}
		canonical, err := Marshal(content)
		if err != nil || len(canonical) > 65536 || bytes.HasSuffix(canonical, []byte("\n")) {
			t.Fatalf("canonical length=%d err=%v", len(canonical), err)
		}
		reparsed, err := Parse(canonical)
		if err != nil || !reflect.DeepEqual(reparsed, content) {
			t.Fatalf("reparse mismatch err=%v got=%+v want=%+v", err, reparsed, content)
		}
		again, err := Marshal(reparsed)
		if err != nil || !bytes.Equal(again, canonical) {
			t.Fatalf("canonical instability err=%v first=%q second=%q", err, canonical, again)
		}
	})
}
