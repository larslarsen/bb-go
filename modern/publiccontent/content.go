// Package publiccontent defines the bounded canonical public rich-content document.
package publiccontent

import (
	"bytes"
	"encoding/json"
	"errors"
	"io"
	"net/url"
	"strconv"
	"strings"
	"unicode"
	"unicode/utf8"

	"github.com/ipfs/go-cid"
	mh "github.com/multiformats/go-multihash"
)

const (
	SchemaV1            = "bitbook.public-content/1"
	maxDocumentBytes    = 65536
	maxContainerDepth   = 8
	maxParagraphs       = 32
	maxInlineNodes      = 256
	maxAttachments      = 8
	maxPlainTextScalars = 280
	maxImageBytes       = uint64(25 << 20)
	maxVideoBytes       = uint64(100 << 20)
	maxAggregateBytes   = uint64(200 << 20)
	maxDimension        = uint64(8192)
	maxPixels           = uint64(40000000)
	maxFilenameScalars  = 255
	maxFilenameBytes    = 1024
	maxLinkBytes        = 2048
	maxCIDBytes         = 128
	maxGIFDurationMS    = uint64(60000)
	maxVideoDurationMS  = uint64(600000)
)

var (
	ErrInvalidContent    = errors.New("invalid public content")
	ErrUnsupportedSchema = errors.New("unsupported public content schema")
)

type Content struct {
	Schema      string                 `json:"schema"`
	Body        []Paragraph            `json:"body"`
	Attachments []AttachmentDescriptor `json:"attachments"`
}

type Paragraph struct {
	Type     string       `json:"type"`
	Children []InlineNode `json:"children"`
}

type InlineNode struct {
	Type         string   `json:"type"`
	Text         string   `json:"text,omitempty"`
	Marks        []string `json:"marks,omitempty"`
	Href         string   `json:"href,omitempty"`
	AttachmentID string   `json:"attachmentId,omitempty"`
}

type AttachmentDescriptor struct {
	AttachmentID string  `json:"attachmentId"`
	CID          string  `json:"cid"`
	MediaType    string  `json:"mediaType"`
	ByteLength   uint64  `json:"byteLength"`
	Width        uint64  `json:"width"`
	Height       uint64  `json:"height"`
	DurationMS   *uint64 `json:"durationMs"`
	Filename     string  `json:"filename"`
}

func Parse(raw []byte) (Content, error) {
	if len(raw) == 0 || len(raw) > maxDocumentBytes || !utf8.Valid(raw) || !validJSONStringEscapes(raw) {
		return Content{}, ErrInvalidContent
	}
	if err := preflightJSON(raw); err != nil {
		return Content{}, err
	}
	parser := jsonParser{decoder: json.NewDecoder(bytes.NewReader(raw))}
	parser.decoder.UseNumber()
	parsed, err := parser.parseContent()
	if err != nil {
		return Content{}, err
	}
	if token, err := parser.decoder.Token(); err != io.EOF || token != nil {
		return Content{}, ErrInvalidContent
	}
	normalized, _, err := normalizeContent(parsed)
	if err != nil {
		return Content{}, err
	}
	encoded, err := marshalNormalized(normalized)
	if err != nil || len(encoded) > maxDocumentBytes {
		return Content{}, ErrInvalidContent
	}
	return normalized, nil
}

func Marshal(content Content) ([]byte, error) {
	normalized, _, err := normalizeContent(content)
	if err != nil {
		return nil, err
	}
	encoded, err := marshalNormalized(normalized)
	if err != nil || len(encoded) > maxDocumentBytes {
		return nil, ErrInvalidContent
	}
	return encoded, nil
}

func PlainText(content Content) (string, error) {
	_, plain, err := normalizeContent(content)
	if err != nil {
		return "", err
	}
	return plain, nil
}

type jsonParser struct {
	decoder *json.Decoder
}

func (p *jsonParser) parseContent() (Content, error) {
	if !p.openObject() {
		return Content{}, ErrInvalidContent
	}
	var content Content
	seen := make(map[string]bool, 3)
	totalChildren := 0
	for p.decoder.More() {
		key, ok := p.nextString()
		if !ok || seen[key] {
			return Content{}, ErrInvalidContent
		}
		seen[key] = true
		switch key {
		case "schema":
			value, ok := p.nextString()
			if !ok {
				return Content{}, ErrInvalidContent
			}
			content.Schema = value
		case "body":
			body, err := p.parseBody(&totalChildren)
			if err != nil {
				return Content{}, err
			}
			content.Body = body
		case "attachments":
			attachments, err := p.parseAttachments()
			if err != nil {
				return Content{}, err
			}
			content.Attachments = attachments
		default:
			return Content{}, ErrInvalidContent
		}
	}
	if !p.closeObject() || !seen["schema"] || !seen["body"] || !seen["attachments"] {
		return Content{}, ErrInvalidContent
	}
	if err := validateSchema(content.Schema); err != nil {
		return Content{}, err
	}
	return content, nil
}

func (p *jsonParser) parseBody(totalChildren *int) ([]Paragraph, error) {
	if !p.openArray() {
		return nil, ErrInvalidContent
	}
	result := make([]Paragraph, 0, 1)
	for p.decoder.More() {
		if len(result) >= maxParagraphs {
			return nil, ErrInvalidContent
		}
		paragraph, err := p.parseParagraph(totalChildren)
		if err != nil {
			return nil, err
		}
		result = append(result, paragraph)
	}
	if !p.closeArray() || len(result) == 0 {
		return nil, ErrInvalidContent
	}
	return result, nil
}

func (p *jsonParser) parseParagraph(totalChildren *int) (Paragraph, error) {
	if !p.openObject() {
		return Paragraph{}, ErrInvalidContent
	}
	paragraph := Paragraph{}
	seen := make(map[string]bool, 2)
	for p.decoder.More() {
		key, ok := p.nextString()
		if !ok || seen[key] {
			return Paragraph{}, ErrInvalidContent
		}
		seen[key] = true
		switch key {
		case "type":
			value, ok := p.nextString()
			if !ok {
				return Paragraph{}, ErrInvalidContent
			}
			paragraph.Type = value
		case "children":
			children, err := p.parseChildren(totalChildren)
			if err != nil {
				return Paragraph{}, err
			}
			paragraph.Children = children
		default:
			return Paragraph{}, ErrInvalidContent
		}
	}
	if !p.closeObject() || !seen["type"] || !seen["children"] {
		return Paragraph{}, ErrInvalidContent
	}
	return paragraph, nil
}

func (p *jsonParser) parseChildren(totalChildren *int) ([]InlineNode, error) {
	if !p.openArray() {
		return nil, ErrInvalidContent
	}
	result := make([]InlineNode, 0)
	for p.decoder.More() {
		if *totalChildren >= maxInlineNodes {
			return nil, ErrInvalidContent
		}
		node, err := p.parseInlineNode()
		if err != nil {
			return nil, err
		}
		*totalChildren = *totalChildren + 1
		result = append(result, node)
	}
	if !p.closeArray() {
		return nil, ErrInvalidContent
	}
	return result, nil
}

func (p *jsonParser) parseInlineNode() (InlineNode, error) {
	if !p.openObject() {
		return InlineNode{}, ErrInvalidContent
	}
	node := InlineNode{}
	seen := make(map[string]bool, 5)
	for p.decoder.More() {
		key, ok := p.nextString()
		if !ok || seen[key] {
			return InlineNode{}, ErrInvalidContent
		}
		seen[key] = true
		switch key {
		case "type":
			value, ok := p.nextString()
			if !ok {
				return InlineNode{}, ErrInvalidContent
			}
			node.Type = value
		case "text":
			value, ok := p.nextString()
			if !ok {
				return InlineNode{}, ErrInvalidContent
			}
			node.Text = value
		case "marks":
			marks, err := p.parseMarks()
			if err != nil {
				return InlineNode{}, err
			}
			node.Marks = marks
		case "href":
			value, ok := p.nextString()
			if !ok {
				return InlineNode{}, ErrInvalidContent
			}
			node.Href = value
		case "attachmentId":
			value, ok := p.nextString()
			if !ok {
				return InlineNode{}, ErrInvalidContent
			}
			node.AttachmentID = value
		default:
			return InlineNode{}, ErrInvalidContent
		}
	}
	if !p.closeObject() || !seen["type"] {
		return InlineNode{}, ErrInvalidContent
	}
	switch node.Type {
	case "text":
		if !seen["text"] || !seen["marks"] || !seen["href"] || seen["attachmentId"] {
			return InlineNode{}, ErrInvalidContent
		}
	case "break":
		if len(seen) != 1 {
			return InlineNode{}, ErrInvalidContent
		}
	case "attachment":
		if !seen["attachmentId"] || len(seen) != 2 {
			return InlineNode{}, ErrInvalidContent
		}
	default:
		return InlineNode{}, ErrInvalidContent
	}
	return node, nil
}

func (p *jsonParser) parseMarks() ([]string, error) {
	if !p.openArray() {
		return nil, ErrInvalidContent
	}
	result := make([]string, 0)
	for p.decoder.More() {
		if len(result) >= 3 {
			return nil, ErrInvalidContent
		}
		mark, ok := p.nextString()
		if !ok {
			return nil, ErrInvalidContent
		}
		result = append(result, mark)
	}
	if !p.closeArray() {
		return nil, ErrInvalidContent
	}
	return result, nil
}

func (p *jsonParser) parseAttachments() ([]AttachmentDescriptor, error) {
	if !p.openArray() {
		return nil, ErrInvalidContent
	}
	result := make([]AttachmentDescriptor, 0)
	for p.decoder.More() {
		if len(result) >= maxAttachments {
			return nil, ErrInvalidContent
		}
		descriptor, err := p.parseAttachment()
		if err != nil {
			return nil, err
		}
		result = append(result, descriptor)
	}
	if !p.closeArray() {
		return nil, ErrInvalidContent
	}
	return result, nil
}

func (p *jsonParser) parseAttachment() (AttachmentDescriptor, error) {
	if !p.openObject() {
		return AttachmentDescriptor{}, ErrInvalidContent
	}
	descriptor := AttachmentDescriptor{}
	seen := make(map[string]bool, 8)
	for p.decoder.More() {
		key, ok := p.nextString()
		if !ok || seen[key] {
			return AttachmentDescriptor{}, ErrInvalidContent
		}
		seen[key] = true
		switch key {
		case "attachmentId":
			value, ok := p.nextString()
			if !ok {
				return AttachmentDescriptor{}, ErrInvalidContent
			}
			descriptor.AttachmentID = value
		case "cid":
			value, ok := p.nextString()
			if !ok {
				return AttachmentDescriptor{}, ErrInvalidContent
			}
			descriptor.CID = value
		case "mediaType":
			value, ok := p.nextString()
			if !ok {
				return AttachmentDescriptor{}, ErrInvalidContent
			}
			descriptor.MediaType = value
		case "byteLength":
			value, ok := p.nextUint()
			if !ok {
				return AttachmentDescriptor{}, ErrInvalidContent
			}
			descriptor.ByteLength = value
		case "width":
			value, ok := p.nextUint()
			if !ok {
				return AttachmentDescriptor{}, ErrInvalidContent
			}
			descriptor.Width = value
		case "height":
			value, ok := p.nextUint()
			if !ok {
				return AttachmentDescriptor{}, ErrInvalidContent
			}
			descriptor.Height = value
		case "durationMs":
			token, err := p.decoder.Token()
			if err != nil {
				return AttachmentDescriptor{}, ErrInvalidContent
			}
			if token == nil {
				descriptor.DurationMS = nil
			} else {
				value, ok := parseUintToken(token)
				if !ok {
					return AttachmentDescriptor{}, ErrInvalidContent
				}
				descriptor.DurationMS = &value
			}
		case "filename":
			value, ok := p.nextString()
			if !ok {
				return AttachmentDescriptor{}, ErrInvalidContent
			}
			descriptor.Filename = value
		default:
			return AttachmentDescriptor{}, ErrInvalidContent
		}
	}
	if !p.closeObject() || len(seen) != 8 {
		return AttachmentDescriptor{}, ErrInvalidContent
	}
	return descriptor, nil
}

func (p *jsonParser) nextString() (string, bool) {
	token, err := p.decoder.Token()
	if err != nil {
		return "", false
	}
	value, ok := token.(string)
	return value, ok
}

func (p *jsonParser) nextUint() (uint64, bool) {
	token, err := p.decoder.Token()
	if err != nil {
		return 0, false
	}
	return parseUintToken(token)
}

func parseUintToken(token any) (uint64, bool) {
	number, ok := token.(json.Number)
	if !ok {
		return 0, false
	}
	text := number.String()
	if text == "" || (len(text) > 1 && text[0] == '0') {
		return 0, false
	}
	for index := 0; index < len(text); index++ {
		if text[index] < '0' || text[index] > '9' {
			return 0, false
		}
	}
	value, err := strconv.ParseUint(text, 10, 64)
	return value, err == nil
}

func (p *jsonParser) openObject() bool  { return p.expectDelimiter('{') }
func (p *jsonParser) closeObject() bool { return p.expectDelimiter('}') }
func (p *jsonParser) openArray() bool   { return p.expectDelimiter('[') }
func (p *jsonParser) closeArray() bool  { return p.expectDelimiter(']') }

func (p *jsonParser) expectDelimiter(want json.Delim) bool {
	token, err := p.decoder.Token()
	if err != nil {
		return false
	}
	delimiter, ok := token.(json.Delim)
	return ok && delimiter == want
}

func preflightJSON(raw []byte) error {
	decoder := json.NewDecoder(bytes.NewReader(raw))
	decoder.UseNumber()
	if err := scanJSONValue(decoder, 0); err != nil {
		return ErrInvalidContent
	}
	if token, err := decoder.Token(); err != io.EOF || token != nil {
		return ErrInvalidContent
	}
	return nil
}

func scanJSONValue(decoder *json.Decoder, depth int) error {
	token, err := decoder.Token()
	if err != nil {
		return err
	}
	delimiter, container := token.(json.Delim)
	if !container {
		return nil
	}
	if depth+1 > maxContainerDepth {
		return ErrInvalidContent
	}
	switch delimiter {
	case '{':
		seen := make(map[string]struct{})
		for decoder.More() {
			keyToken, err := decoder.Token()
			if err != nil {
				return err
			}
			key, ok := keyToken.(string)
			if !ok {
				return ErrInvalidContent
			}
			if _, exists := seen[key]; exists {
				return ErrInvalidContent
			}
			seen[key] = struct{}{}
			if err := scanJSONValue(decoder, depth+1); err != nil {
				return err
			}
		}
		closing, err := decoder.Token()
		if err != nil || closing != json.Delim('}') {
			return ErrInvalidContent
		}
	case '[':
		for decoder.More() {
			if err := scanJSONValue(decoder, depth+1); err != nil {
				return err
			}
		}
		closing, err := decoder.Token()
		if err != nil || closing != json.Delim(']') {
			return ErrInvalidContent
		}
	default:
		return ErrInvalidContent
	}
	return nil
}

func validJSONStringEscapes(raw []byte) bool {
	for index := 0; index < len(raw); index++ {
		if raw[index] != '"' {
			continue
		}
		index++
		for index < len(raw) && raw[index] != '"' {
			if raw[index] < 0x20 {
				return false
			}
			if raw[index] != '\\' {
				index++
				continue
			}
			if index+1 >= len(raw) {
				return false
			}
			escape := raw[index+1]
			if escape != 'u' {
				if !strings.ContainsRune(`"\\/bfnrt`, rune(escape)) {
					return false
				}
				index += 2
				continue
			}
			value, ok := parseHex16(raw, index+2)
			if !ok {
				return false
			}
			index += 6
			switch {
			case value >= 0xd800 && value <= 0xdbff:
				if index+6 > len(raw) || raw[index] != '\\' || raw[index+1] != 'u' {
					return false
				}
				low, ok := parseHex16(raw, index+2)
				if !ok || low < 0xdc00 || low > 0xdfff {
					return false
				}
				index += 6
			case value >= 0xdc00 && value <= 0xdfff:
				return false
			}
		}
		if index >= len(raw) {
			return false
		}
	}
	return true
}

func parseHex16(raw []byte, start int) (uint16, bool) {
	if start+4 > len(raw) {
		return 0, false
	}
	var value uint16
	for index := start; index < start+4; index++ {
		var digit byte
		switch character := raw[index]; {
		case character >= '0' && character <= '9':
			digit = character - '0'
		case character >= 'a' && character <= 'f':
			digit = character - 'a' + 10
		case character >= 'A' && character <= 'F':
			digit = character - 'A' + 10
		default:
			return 0, false
		}
		value = value<<4 | uint16(digit)
	}
	return value, true
}

func normalizeContent(content Content) (Content, string, error) {
	if err := validateSchema(content.Schema); err != nil {
		return Content{}, "", err
	}
	if len(content.Body) < 1 || len(content.Body) > maxParagraphs || len(content.Attachments) > maxAttachments {
		return Content{}, "", ErrInvalidContent
	}
	totalChildren := 0
	for _, paragraph := range content.Body {
		if len(paragraph.Children) > maxInlineNodes-totalChildren {
			return Content{}, "", ErrInvalidContent
		}
		totalChildren += len(paragraph.Children)
	}
	normalized := Content{
		Schema:      SchemaV1,
		Body:        make([]Paragraph, len(content.Body)),
		Attachments: make([]AttachmentDescriptor, len(content.Attachments)),
	}
	var plain strings.Builder
	plainScalars := 0
	hasNonWhitespace := false
	hasAttachment := false
	references := make(map[string]int)
	for paragraphIndex, paragraph := range content.Body {
		if paragraph.Type != "paragraph" {
			return Content{}, "", ErrInvalidContent
		}
		if paragraphIndex > 0 {
			if plainScalars >= maxPlainTextScalars {
				return Content{}, "", ErrInvalidContent
			}
			plain.WriteByte('\n')
			plainScalars++
		}
		normalizedParagraph := Paragraph{Type: "paragraph", Children: make([]InlineNode, len(paragraph.Children))}
		for nodeIndex, node := range paragraph.Children {
			switch node.Type {
			case "text":
				if node.Text == "" || node.AttachmentID != "" || len(node.Marks) > 3 || !validText(node.Text) || !validSafeLink(node.Href) {
					return Content{}, "", ErrInvalidContent
				}
				textScalars := utf8.RuneCountInString(node.Text)
				if textScalars > maxPlainTextScalars-plainScalars {
					return Content{}, "", ErrInvalidContent
				}
				marks, ok := normalizeMarks(node.Marks)
				if !ok {
					return Content{}, "", ErrInvalidContent
				}
				plain.WriteString(node.Text)
				plainScalars += textScalars
				for _, character := range node.Text {
					if !unicode.IsSpace(character) {
						hasNonWhitespace = true
					}
				}
				normalizedParagraph.Children[nodeIndex] = InlineNode{Type: "text", Text: node.Text, Marks: marks, Href: node.Href}
			case "break":
				if node.Text != "" || node.Marks != nil || node.Href != "" || node.AttachmentID != "" {
					return Content{}, "", ErrInvalidContent
				}
				if plainScalars >= maxPlainTextScalars {
					return Content{}, "", ErrInvalidContent
				}
				plain.WriteByte('\n')
				plainScalars++
				normalizedParagraph.Children[nodeIndex] = InlineNode{Type: "break"}
			case "attachment":
				if node.Text != "" || node.Marks != nil || node.Href != "" || !validAttachmentID(node.AttachmentID) {
					return Content{}, "", ErrInvalidContent
				}
				references[node.AttachmentID]++
				if references[node.AttachmentID] != 1 {
					return Content{}, "", ErrInvalidContent
				}
				hasAttachment = true
				normalizedParagraph.Children[nodeIndex] = InlineNode{Type: "attachment", AttachmentID: node.AttachmentID}
			default:
				return Content{}, "", ErrInvalidContent
			}
		}
		normalized.Body[paragraphIndex] = normalizedParagraph
	}
	projection := plain.String()
	if !hasNonWhitespace && !hasAttachment {
		return Content{}, "", ErrInvalidContent
	}

	descriptorIDs := make(map[string]struct{}, len(content.Attachments))
	var aggregate uint64
	for index, descriptor := range content.Attachments {
		if !validAttachmentID(descriptor.AttachmentID) || !validCID(descriptor.CID) || !validFilename(descriptor.Filename) {
			return Content{}, "", ErrInvalidContent
		}
		if _, exists := descriptorIDs[descriptor.AttachmentID]; exists || references[descriptor.AttachmentID] != 1 {
			return Content{}, "", ErrInvalidContent
		}
		descriptorIDs[descriptor.AttachmentID] = struct{}{}
		if !validDescriptorBounds(descriptor) || descriptor.ByteLength > maxAggregateBytes-aggregate {
			return Content{}, "", ErrInvalidContent
		}
		aggregate += descriptor.ByteLength
		copyDescriptor := descriptor
		if descriptor.DurationMS != nil {
			value := *descriptor.DurationMS
			copyDescriptor.DurationMS = &value
		}
		normalized.Attachments[index] = copyDescriptor
	}
	if len(references) != len(descriptorIDs) {
		return Content{}, "", ErrInvalidContent
	}
	if size, ok := canonicalDocumentSize(normalized); !ok || size > maxDocumentBytes {
		return Content{}, "", ErrInvalidContent
	}
	return normalized, projection, nil
}

func validateSchema(schema string) error {
	if schema == SchemaV1 {
		return nil
	}
	if schema == "" || !utf8.ValidString(schema) {
		return ErrInvalidContent
	}
	return ErrUnsupportedSchema
}

func normalizeMarks(marks []string) ([]string, bool) {
	present := make(map[string]bool, 3)
	for _, mark := range marks {
		switch mark {
		case "bold", "italic", "code":
		default:
			return nil, false
		}
		if present[mark] {
			return nil, false
		}
		present[mark] = true
	}
	result := make([]string, 0, len(marks))
	for _, mark := range []string{"bold", "italic", "code"} {
		if present[mark] {
			result = append(result, mark)
		}
	}
	return result, true
}

func validText(text string) bool {
	if !utf8.ValidString(text) {
		return false
	}
	for _, character := range text {
		if character <= 0x1f && character != '\t' {
			return false
		}
	}
	return true
}

func validAttachmentID(value string) bool {
	if len(value) != 32 || value == "00000000000000000000000000000000" {
		return false
	}
	for index := 0; index < len(value); index++ {
		if (value[index] < '0' || value[index] > '9') && (value[index] < 'a' || value[index] > 'f') {
			return false
		}
	}
	return true
}

func validCID(value string) bool {
	if len(value) == 0 || len(value) > maxCIDBytes || !asciiOnly(value) || value[0] != 'b' {
		return false
	}
	decoded, err := cid.Decode(value)
	if err != nil || decoded.Version() != 1 || decoded.String() != value || (decoded.Type() != cid.Raw && decoded.Type() != cid.DagProtobuf) {
		return false
	}
	digest, err := mh.Decode(decoded.Hash())
	return err == nil && digest.Code == mh.SHA2_256 && digest.Length == 32 && len(digest.Digest) == 32
}

func validDescriptorBounds(descriptor AttachmentDescriptor) bool {
	if descriptor.ByteLength == 0 || descriptor.Width == 0 || descriptor.Width > maxDimension || descriptor.Height == 0 || descriptor.Height > maxDimension ||
		descriptor.Width > maxPixels/descriptor.Height {
		return false
	}
	switch descriptor.MediaType {
	case "image/jpeg", "image/png", "image/webp":
		return descriptor.ByteLength <= maxImageBytes && descriptor.DurationMS == nil
	case "image/gif":
		return descriptor.ByteLength <= maxImageBytes && descriptor.DurationMS != nil && *descriptor.DurationMS >= 1 && *descriptor.DurationMS <= maxGIFDurationMS
	case "video/mp4", "video/webm":
		return descriptor.ByteLength <= maxVideoBytes && descriptor.DurationMS != nil && *descriptor.DurationMS >= 1 && *descriptor.DurationMS <= maxVideoDurationMS
	default:
		return false
	}
}

func validFilename(value string) bool {
	if !utf8.ValidString(value) || len(value) > maxFilenameBytes || utf8.RuneCountInString(value) > maxFilenameScalars {
		return false
	}
	for _, character := range value {
		if character <= 0x1f || character == 0x7f || character == '/' || character == '\\' {
			return false
		}
	}
	return true
}

func validSafeLink(value string) bool {
	if value == "" {
		return true
	}
	if len(value) > maxLinkBytes || !asciiOnly(value) || (!strings.HasPrefix(value, "http://") && !strings.HasPrefix(value, "https://")) {
		return false
	}
	for index := 0; index < len(value); index++ {
		character := value[index]
		if character <= 0x20 || character == 0x7f || character == '\\' {
			return false
		}
		if character == '%' {
			if index+2 >= len(value) {
				return false
			}
			encoded, ok := parseHexByte(value[index+1], value[index+2])
			if !ok || encoded <= 0x1f || encoded == 0x7f || encoded == '\\' {
				return false
			}
			index += 2
		}
	}
	parsed, err := url.Parse(value)
	return err == nil && parsed.Opaque == "" && parsed.IsAbs() && (parsed.Scheme == "http" || parsed.Scheme == "https") &&
		parsed.Host != "" && parsed.Hostname() != "" && parsed.User == nil
}

func parseHexByte(first, second byte) (byte, bool) {
	high, ok := hexNibble(first)
	if !ok {
		return 0, false
	}
	low, ok := hexNibble(second)
	return high<<4 | low, ok
}

func hexNibble(character byte) (byte, bool) {
	switch {
	case character >= '0' && character <= '9':
		return character - '0', true
	case character >= 'a' && character <= 'f':
		return character - 'a' + 10, true
	case character >= 'A' && character <= 'F':
		return character - 'A' + 10, true
	default:
		return 0, false
	}
}

func asciiOnly(value string) bool {
	for index := 0; index < len(value); index++ {
		if value[index] > 0x7f {
			return false
		}
	}
	return true
}

func canonicalDocumentSize(content Content) (int, bool) {
	size := 0
	add := func(amount int) bool {
		if amount < 0 || size > maxDocumentBytes-amount {
			return false
		}
		size += amount
		return true
	}
	addString := func(value string) bool { return add(jsonStringSize(value)) }
	if !add(len(`{"schema":`)) || !addString(content.Schema) || !add(len(`,"body":[`)) {
		return 0, false
	}
	for paragraphIndex, paragraph := range content.Body {
		if paragraphIndex > 0 && !add(1) {
			return 0, false
		}
		if !add(len(`{"type":"paragraph","children":[`)) {
			return 0, false
		}
		for nodeIndex, node := range paragraph.Children {
			if nodeIndex > 0 && !add(1) {
				return 0, false
			}
			switch node.Type {
			case "text":
				if !add(len(`{"type":"text","text":`)) || !addString(node.Text) || !add(len(`,"marks":[`)) {
					return 0, false
				}
				for markIndex, mark := range node.Marks {
					if markIndex > 0 && !add(1) {
						return 0, false
					}
					if !addString(mark) {
						return 0, false
					}
				}
				if !add(len(`],"href":`)) || !addString(node.Href) || !add(1) {
					return 0, false
				}
			case "break":
				if !add(len(`{"type":"break"}`)) {
					return 0, false
				}
			case "attachment":
				if !add(len(`{"type":"attachment","attachmentId":`)) || !addString(node.AttachmentID) || !add(1) {
					return 0, false
				}
			default:
				return 0, false
			}
		}
		if !add(2) {
			return 0, false
		}
	}
	if !add(len(`],"attachments":[`)) {
		return 0, false
	}
	for index, descriptor := range content.Attachments {
		if index > 0 && !add(1) {
			return 0, false
		}
		if !add(len(`{"attachmentId":`)) || !addString(descriptor.AttachmentID) ||
			!add(len(`,"cid":`)) || !addString(descriptor.CID) ||
			!add(len(`,"mediaType":`)) || !addString(descriptor.MediaType) ||
			!add(len(`,"byteLength":`)) || !add(decimalUintSize(descriptor.ByteLength)) ||
			!add(len(`,"width":`)) || !add(decimalUintSize(descriptor.Width)) ||
			!add(len(`,"height":`)) || !add(decimalUintSize(descriptor.Height)) ||
			!add(len(`,"durationMs":`)) {
			return 0, false
		}
		if descriptor.DurationMS == nil {
			if !add(len("null")) {
				return 0, false
			}
		} else if !add(decimalUintSize(*descriptor.DurationMS)) {
			return 0, false
		}
		if !add(len(`,"filename":`)) || !addString(descriptor.Filename) || !add(1) {
			return 0, false
		}
	}
	if !add(2) {
		return 0, false
	}
	return size, true
}

func jsonStringSize(value string) int {
	size := 2
	for index := 0; index < len(value); {
		character := value[index]
		if character < utf8.RuneSelf {
			switch character {
			case '"', '\\', '\b', '\f', '\n', '\r', '\t':
				size += 2
			case '<', '>', '&':
				size += 6
			default:
				if character < 0x20 {
					size += 6
				} else {
					size++
				}
			}
			index++
			continue
		}
		decoded, width := utf8.DecodeRuneInString(value[index:])
		if decoded == '\u2028' || decoded == '\u2029' {
			size += 6
		} else {
			size += width
		}
		index += width
	}
	return size
}

func decimalUintSize(value uint64) int {
	size := 1
	for value >= 10 {
		value /= 10
		size++
	}
	return size
}

type canonicalContent struct {
	Schema      string                          `json:"schema"`
	Body        []canonicalParagraph            `json:"body"`
	Attachments []canonicalAttachmentDescriptor `json:"attachments"`
}

type canonicalParagraph struct {
	Type     string          `json:"type"`
	Children []canonicalNode `json:"children"`
}

type canonicalNode struct {
	node InlineNode
}

type canonicalAttachmentDescriptor struct {
	AttachmentID string  `json:"attachmentId"`
	CID          string  `json:"cid"`
	MediaType    string  `json:"mediaType"`
	ByteLength   uint64  `json:"byteLength"`
	Width        uint64  `json:"width"`
	Height       uint64  `json:"height"`
	DurationMS   *uint64 `json:"durationMs"`
	Filename     string  `json:"filename"`
}

func (n canonicalNode) MarshalJSON() ([]byte, error) {
	switch n.node.Type {
	case "text":
		return json.Marshal(struct {
			Type  string   `json:"type"`
			Text  string   `json:"text"`
			Marks []string `json:"marks"`
			Href  string   `json:"href"`
		}{Type: "text", Text: n.node.Text, Marks: n.node.Marks, Href: n.node.Href})
	case "break":
		return []byte(`{"type":"break"}`), nil
	case "attachment":
		return json.Marshal(struct {
			Type         string `json:"type"`
			AttachmentID string `json:"attachmentId"`
		}{Type: "attachment", AttachmentID: n.node.AttachmentID})
	default:
		return nil, ErrInvalidContent
	}
}

func marshalNormalized(content Content) ([]byte, error) {
	wire := canonicalContent{
		Schema:      SchemaV1,
		Body:        make([]canonicalParagraph, len(content.Body)),
		Attachments: make([]canonicalAttachmentDescriptor, len(content.Attachments)),
	}
	for paragraphIndex, paragraph := range content.Body {
		children := make([]canonicalNode, len(paragraph.Children))
		for nodeIndex, node := range paragraph.Children {
			children[nodeIndex] = canonicalNode{node: node}
		}
		wire.Body[paragraphIndex] = canonicalParagraph{Type: "paragraph", Children: children}
	}
	for index, descriptor := range content.Attachments {
		wire.Attachments[index] = canonicalAttachmentDescriptor{
			AttachmentID: descriptor.AttachmentID,
			CID:          descriptor.CID,
			MediaType:    descriptor.MediaType,
			ByteLength:   descriptor.ByteLength,
			Width:        descriptor.Width,
			Height:       descriptor.Height,
			DurationMS:   descriptor.DurationMS,
			Filename:     descriptor.Filename,
		}
	}
	return json.Marshal(wire)
}
