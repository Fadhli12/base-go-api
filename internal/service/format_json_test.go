package service

import (
	"bytes"
	"context"
	"fmt"
	"strings"
	"testing"

	"github.com/example/go-api-base/internal/domain"
	"github.com/google/uuid"
)

type mockExportable struct {
	entityType string
	data       map[string]interface{}
}

func (m *mockExportable) ToExportRecord() map[string]interface{} {
	return m.data
}

func (m *mockExportable) GetEntityType() string {
	return m.entityType
}

type mockCursor struct {
	batches  [][]Exportable
	index    int
	hasMore  bool
	closed   bool
	nextErr  error
	batchSize int
}

func (m *mockCursor) Next(ctx context.Context, batchSize int) ([]Exportable, error) {
	m.batchSize = batchSize
	if m.nextErr != nil {
		return nil, m.nextErr
	}
	if m.index >= len(m.batches) {
		m.hasMore = false
		return nil, nil
	}
	batch := m.batches[m.index]
	m.index++
	m.hasMore = m.index < len(m.batches)
	return batch, nil
}

func (m *mockCursor) HasMore() bool {
	return m.hasMore
}

func (m *mockCursor) Close() error {
	m.closed = true
	return nil
}

func makeEntity(entityType, id string, extra map[string]interface{}) *mockExportable {
	data := map[string]interface{}{
		"id":   id,
		"name": entityType + "_" + id,
	}
	for k, v := range extra {
		data[k] = v
	}
	return &mockExportable{entityType: entityType, data: data}
}

func newRegistry() *domain.EntityRegistry {
	return domain.NewEntityRegistry()
}

func TestJSONEncoder_ContentType(t *testing.T) {
	enc := NewJSONEncoder(newRegistry())
	if got := enc.ContentType(); got != "application/x-ndjson" {
		t.Errorf("ContentType() = %q, want %q", got, "application/x-ndjson")
	}
}

func TestJSONEncoder_FileExtension(t *testing.T) {
	enc := NewJSONEncoder(newRegistry())
	if got := enc.FileExtension(); got != "json" {
		t.Errorf("FileExtension() = %q, want %q", got, "json")
	}
}

func TestJSONEncoder_Encode_WritesNDJSONFormat(t *testing.T) {
	registry := newRegistry()
	enc := NewJSONEncoder(registry)

	entities := []Exportable{
		makeEntity("organizations", uuid.New().String(), nil),
		makeEntity("users", uuid.New().String(), nil),
	}
	cursor := &mockCursor{
		batches: [][]Exportable{entities},
		hasMore: true,
	}

	var buf bytes.Buffer
	if err := enc.Encode(context.Background(), cursor, &buf); err != nil {
		t.Fatalf("Encode() error = %v", err)
	}

	lines := strings.Split(strings.TrimSpace(buf.String()), "\n")
	if len(lines) != 2 {
		t.Fatalf("expected 2 lines, got %d", len(lines))
	}

	for i, line := range lines {
		if !strings.HasPrefix(line, "{") || !strings.HasSuffix(line, "}") {
			t.Errorf("line %d: expected JSON object, got %q", i, line)
		}
	}

	first := lines[0]
	if !strings.Contains(first, `"type":"organizations"`) && !strings.Contains(first, `"type": "organizations"`) {
		if !strings.Contains(first, `"organizations"`) {
			t.Errorf("first line should contain type 'organizations': %s", first)
		}
	}
}

func TestJSONEncoder_Encode_KeysetPagination(t *testing.T) {
	registry := newRegistry()
	enc := NewJSONEncoder(registry)

	batch1 := []Exportable{makeEntity("organizations", "1", nil)}
	batch2 := []Exportable{makeEntity("users", "2", nil)}
	cursor := &mockCursor{
		batches: [][]Exportable{batch1, batch2},
		hasMore: true,
	}

	var buf bytes.Buffer
	if err := enc.Encode(context.Background(), cursor, &buf); err != nil {
		t.Fatalf("Encode() error = %v", err)
	}

	if cursor.batchSize != encoderBatchSize {
		t.Errorf("expected batchSize %d, got %d", encoderBatchSize, cursor.batchSize)
	}

	lines := strings.Split(strings.TrimSpace(buf.String()), "\n")
	if len(lines) != 2 {
		t.Fatalf("expected 2 lines, got %d", len(lines))
	}
}

func TestJSONEncoder_Encode_ContextCancellation(t *testing.T) {
	registry := newRegistry()
	enc := NewJSONEncoder(registry)

	ctx, cancel := context.WithCancel(context.Background())
	cancel()

	entities := []Exportable{makeEntity("organizations", uuid.New().String(), nil)}
	cursor := &mockCursor{
		batches: [][]Exportable{entities},
		hasMore: true,
	}

	var buf bytes.Buffer
	err := enc.Encode(ctx, cursor, &buf)
	if err != context.Canceled {
		t.Errorf("expected context.Canceled, got %v", err)
	}
}

func TestJSONEncoder_Encode_SkipsRestrictedEntities(t *testing.T) {
	registry := newRegistry()
	enc := NewJSONEncoder(registry)

	entities := []Exportable{
		makeEntity("api_keys", uuid.New().String(), nil),
		makeEntity("organizations", uuid.New().String(), nil),
	}
	cursor := &mockCursor{
		batches: [][]Exportable{entities},
		hasMore: true,
	}

	var buf bytes.Buffer
	if err := enc.Encode(context.Background(), cursor, &buf); err != nil {
		t.Fatalf("Encode() error = %v", err)
	}

	lines := strings.Split(strings.TrimSpace(buf.String()), "\n")
	if len(lines) != 1 {
		t.Fatalf("expected 1 line (restricted api_keys skipped), got %d", len(lines))
	}
	if !strings.Contains(lines[0], "organizations") {
		t.Errorf("expected organizations line, got %s", lines[0])
	}
}

func TestJSONEncoder_Encode_CursorError(t *testing.T) {
	registry := newRegistry()
	enc := NewJSONEncoder(registry)

	cursor := &mockCursor{
		batches: [][]Exportable{},
		hasMore: true,
		nextErr: fmt.Errorf("db connection lost"),
	}

	var buf bytes.Buffer
	err := enc.Encode(context.Background(), cursor, &buf)
	if err == nil {
		t.Fatal("expected error from cursor, got nil")
	}
	if !strings.Contains(err.Error(), "cursor next") {
		t.Errorf("expected 'cursor next' error, got %v", err)
	}
}

func TestJSONEncoder_Encode_ClosesCursor(t *testing.T) {
	registry := newRegistry()
	enc := NewJSONEncoder(registry)

	cursor := &mockCursor{
		batches: [][]Exportable{},
		hasMore: false,
	}

	var buf bytes.Buffer
	_ = enc.Encode(context.Background(), cursor, &buf)

	if !cursor.closed {
		t.Error("expected cursor to be closed after Encode")
	}
}

func TestJSONDecoder_ContentType(t *testing.T) {
	dec := NewJSONDecoder(newRegistry())
	if got := dec.ContentType(); got != "application/x-ndjson" {
		t.Errorf("ContentType() = %q, want %q", got, "application/x-ndjson")
	}
}

func TestJSONDecoder_FileExtension(t *testing.T) {
	dec := NewJSONDecoder(newRegistry())
	if got := dec.FileExtension(); got != "json" {
		t.Errorf("FileExtension() = %q, want %q", got, "json")
	}
}

func TestJSONDecoder_CanValidate(t *testing.T) {
	dec := NewJSONDecoder(newRegistry())
	if !dec.CanValidate() {
		t.Error("CanValidate() = false, want true")
	}
}

func TestJSONDecoder_Validate_CorrectCount(t *testing.T) {
	dec := NewJSONDecoder(newRegistry())

	input := `{"type":"organizations","id":"org1","data":{"id":"org1","name":"Org"}}
{"type":"users","id":"user1","data":{"id":"user1","name":"User"}}
{"type":"users","id":"user2","data":{"id":"user2","name":"User2"}}`

	preview, err := dec.Validate(context.Background(), strings.NewReader(input))
	if err != nil {
		t.Fatalf("Validate() error = %v", err)
	}

	if preview.TotalRecords != 3 {
		t.Errorf("TotalRecords = %d, want 3", preview.TotalRecords)
	}
	if preview.RecordsByType["organizations"] != 1 {
		t.Errorf("organizations count = %d, want 1", preview.RecordsByType["organizations"])
	}
	if preview.RecordsByType["users"] != 2 {
		t.Errorf("users count = %d, want 2", preview.RecordsByType["users"])
	}
}

func TestJSONDecoder_Validate_RejectsExceedsLimit(t *testing.T) {
	dec := NewJSONDecoder(newRegistry())

	var lines []string
	for i := 0; i < maxImportEntities+1; i++ {
		id := uuid.New().String()
		lines = append(lines, fmt.Sprintf(`{"type":"users","id":"%s","data":{"id":"%s","name":"u%d"}}`, id, id, i))
	}
	input := strings.Join(lines, "\n")

	preview, err := dec.Validate(context.Background(), strings.NewReader(input))
	if err == nil {
		t.Fatal("expected validation error for exceeding limit, got nil")
	}

	found := false
	for _, e := range preview.ValidationErrors {
		if strings.Contains(e, "exceeds limit") {
			found = true
			break
		}
	}
	if !found {
		t.Errorf("expected 'exceeds limit' error, got %v", preview.ValidationErrors)
	}
}

func TestJSONDecoder_Validate_RejectsInvalidJSON(t *testing.T) {
	dec := NewJSONDecoder(newRegistry())

	input := `{invalid json}`

	preview, err := dec.Validate(context.Background(), strings.NewReader(input))
	if err == nil {
		t.Fatal("expected validation error for invalid JSON, got nil")
	}

	found := false
	for _, e := range preview.ValidationErrors {
		if strings.Contains(e, "invalid JSON") {
			found = true
			break
		}
	}
	if !found {
		t.Errorf("expected 'invalid JSON' error, got %v", preview.ValidationErrors)
	}
}

func TestJSONDecoder_Validate_RejectsMissingType(t *testing.T) {
	dec := NewJSONDecoder(newRegistry())

	input := `{"id":"1","data":{"id":"1","name":"test"}}`

	preview, err := dec.Validate(context.Background(), strings.NewReader(input))
	if err == nil {
		t.Fatal("expected validation error for missing type, got nil")
	}

	found := false
	for _, e := range preview.ValidationErrors {
		if strings.Contains(e, "missing required field 'type'") {
			found = true
			break
		}
	}
	if !found {
		t.Errorf("expected 'missing type' error, got %v", preview.ValidationErrors)
	}
}

func TestJSONDecoder_Validate_RejectsMissingID(t *testing.T) {
	dec := NewJSONDecoder(newRegistry())

	input := `{"type":"users","id":"1","data":{"name":"test"}}`

	preview, err := dec.Validate(context.Background(), strings.NewReader(input))
	if err == nil {
		t.Fatal("expected validation error for missing id in data, got nil")
	}

	found := false
	for _, e := range preview.ValidationErrors {
		if strings.Contains(e, "missing required field 'id' in data") {
			found = true
			break
		}
	}
	if !found {
		t.Errorf("expected 'missing id' error, got %v", preview.ValidationErrors)
	}
}

func TestJSONDecoder_Validate_RejectsMissingData(t *testing.T) {
	dec := NewJSONDecoder(newRegistry())

	input := `{"type":"users","id":"1"}`

	preview, err := dec.Validate(context.Background(), strings.NewReader(input))
	if err == nil {
		t.Fatal("expected validation error for missing data, got nil")
	}

	found := false
	for _, e := range preview.ValidationErrors {
		if strings.Contains(e, "missing required field 'data'") {
			found = true
			break
		}
	}
	if !found {
		t.Errorf("expected 'missing data' error, got %v", preview.ValidationErrors)
	}
}

func TestJSONDecoder_Validate_RejectsRestrictedEntityType(t *testing.T) {
	dec := NewJSONDecoder(newRegistry())

	input := `{"type":"api_keys","id":"1","data":{"id":"1","name":"key"}}`

	preview, err := dec.Validate(context.Background(), strings.NewReader(input))
	if err == nil {
		t.Fatal("expected validation error for restricted entity type, got nil")
	}

	found := false
	for _, e := range preview.ValidationErrors {
		if strings.Contains(e, "restricted") {
			found = true
			break
		}
	}
	if !found {
		t.Errorf("expected 'restricted' error, got %v", preview.ValidationErrors)
	}
}

func TestJSONDecoder_Validate_RejectsUnknownEntityType(t *testing.T) {
	dec := NewJSONDecoder(newRegistry())

	input := `{"type":"unknown_type","id":"1","data":{"id":"1","name":"test"}}`

	preview, err := dec.Validate(context.Background(), strings.NewReader(input))
	if err == nil {
		t.Fatal("expected validation error for unknown entity type, got nil")
	}

	found := false
	for _, e := range preview.ValidationErrors {
		if strings.Contains(e, "unknown entity type") {
			found = true
			break
		}
	}
	if !found {
		t.Errorf("expected 'unknown entity type' error, got %v", preview.ValidationErrors)
	}
}

func TestJSONDecoder_Validate_ContextCancellation(t *testing.T) {
	dec := NewJSONDecoder(newRegistry())

	ctx, cancel := context.WithCancel(context.Background())
	cancel()

	input := `{"type":"users","id":"1","data":{"id":"1","name":"test"}}`
	_, err := dec.Validate(ctx, strings.NewReader(input))
	if err != context.Canceled {
		t.Errorf("expected context.Canceled, got %v", err)
	}
}

func TestJSONDecoder_Decode_YieldsRecordsInTopologicalOrder(t *testing.T) {
	dec := NewJSONDecoder(newRegistry())

	orgID := uuid.New().String()
	userID := uuid.New().String()

	input := fmt.Sprintf(
		`{"type":"organizations","id":"%s","data":{"id":"%s","name":"Org"}}
{"type":"roles","id":"r1","data":{"id":"r1","name":"Admin"}}
{"type":"users","id":"%s","data":{"id":"%s","name":"User"}}`,
		orgID, orgID, userID, userID,
	)

	ch, err := dec.Decode(context.Background(), strings.NewReader(input))
	if err != nil {
		t.Fatalf("Decode() error = %v", err)
	}

	var records []ImportRecord
	for rec := range ch {
		records = append(records, rec)
	}

	if len(records) != 3 {
		t.Fatalf("expected 3 records, got %d", len(records))
	}

	if records[0].EntityType != "organizations" {
		t.Errorf("record 0 type = %q, want organizations", records[0].EntityType)
	}
	if records[1].EntityType != "roles" {
		t.Errorf("record 1 type = %q, want roles", records[1].EntityType)
	}
	if records[2].EntityType != "users" {
		t.Errorf("record 2 type = %q, want users", records[2].EntityType)
	}

	if records[0].ExternalID == uuid.Nil {
		t.Error("expected non-nil ExternalID for record 0")
	}
}

func TestJSONDecoder_Decode_ValidatesEntityTypeOrdering(t *testing.T) {
	dec := NewJSONDecoder(newRegistry())

	orgID := uuid.New().String()
	userID := uuid.New().String()

	input := fmt.Sprintf(
		`{"type":"users","id":"%s","data":{"id":"%s","name":"User"}}
{"type":"organizations","id":"%s","data":{"id":"%s","name":"Org"}}`,
		userID, userID, orgID, orgID,
	)

	ch, err := dec.Decode(context.Background(), strings.NewReader(input))
	if err != nil {
		t.Fatalf("Decode() error = %v", err)
	}

	var records []ImportRecord
	for rec := range ch {
		records = append(records, rec)
	}

	if len(records) != 2 {
		t.Fatalf("expected 2 records, got %d", len(records))
	}

	if len(records[1].Errors) == 0 {
		t.Error("expected topological order error for record 1, got no errors")
	}
	if !strings.Contains(records[1].Errors[0], "out of topological order") {
		t.Errorf("expected 'out of topological order' error, got %v", records[1].Errors)
	}
}

func TestJSONDecoder_Decode_ContextCancellation(t *testing.T) {
	dec := NewJSONDecoder(newRegistry())

	ctx, cancel := context.WithCancel(context.Background())

	input := fmt.Sprintf(
		`{"type":"organizations","id":"%s","data":{"id":"%s","name":"Org"}}`,
		uuid.New().String(), uuid.New().String(),
	)

	ch, err := dec.Decode(ctx, strings.NewReader(input))
	if err != nil {
		t.Fatalf("Decode() error = %v", err)
	}

	cancel()

	records := []ImportRecord{}
	for rec := range ch {
		records = append(records, rec)
		if len(records) >= 1000 {
			break
		}
	}
}

func TestJSONDecoder_Decode_InvalidJSONYieldsError(t *testing.T) {
	dec := NewJSONDecoder(newRegistry())

	input := `{invalid json}`

	ch, err := dec.Decode(context.Background(), strings.NewReader(input))
	if err != nil {
		t.Fatalf("Decode() error = %v", err)
	}

	var records []ImportRecord
	for rec := range ch {
		records = append(records, rec)
	}

	if len(records) != 1 {
		t.Fatalf("expected 1 record, got %d", len(records))
	}
	if len(records[0].Errors) == 0 {
		t.Error("expected parse error in record, got none")
	}
	if !strings.Contains(records[0].Errors[0], "invalid JSON") {
		t.Errorf("expected 'invalid JSON' error, got %v", records[0].Errors)
	}
}

func TestJSONDecoder_Decode_MissingTypeYieldsError(t *testing.T) {
	dec := NewJSONDecoder(newRegistry())

	input := `{"id":"1","data":{"id":"1"}}`

	ch, err := dec.Decode(context.Background(), strings.NewReader(input))
	if err != nil {
		t.Fatalf("Decode() error = %v", err)
	}

	var records []ImportRecord
	for rec := range ch {
		records = append(records, rec)
	}

	if len(records) != 1 {
		t.Fatalf("expected 1 record, got %d", len(records))
	}
	if len(records[0].Errors) == 0 {
		t.Error("expected missing type error, got none")
	}
	if !strings.Contains(records[0].Errors[0], "missing type field") {
		t.Errorf("expected 'missing type field' error, got %v", records[0].Errors)
	}
}

func TestJSONDecoder_Decode_RestrictedEntityTypeYieldsError(t *testing.T) {
	dec := NewJSONDecoder(newRegistry())

	input := `{"type":"api_keys","id":"1","data":{"id":"1","name":"key"}}`

	ch, err := dec.Decode(context.Background(), strings.NewReader(input))
	if err != nil {
		t.Fatalf("Decode() error = %v", err)
	}

	var records []ImportRecord
	for rec := range ch {
		records = append(records, rec)
	}

	if len(records) != 1 {
		t.Fatalf("expected 1 record, got %d", len(records))
	}
	if len(records[0].Errors) == 0 {
		t.Error("expected restricted entity error, got none")
	}
	if !strings.Contains(records[0].Errors[0], "restricted") {
		t.Errorf("expected 'restricted' error, got %v", records[0].Errors)
	}
}

func TestNestingDepth(t *testing.T) {
	tests := []struct {
		name  string
		input interface{}
		want  int
	}{
		{"primitive", "hello", 0},
		{"flat object", map[string]interface{}{"a": 1, "b": "x"}, 1},
		{"nested object", map[string]interface{}{"a": map[string]interface{}{"b": 1}}, 2},
		{"deeply nested", map[string]interface{}{
			"a": map[string]interface{}{
				"b": map[string]interface{}{
					"c": map[string]interface{}{
						"d": 1,
					},
				},
			},
		}, 4},
		{"array", []interface{}{1, 2, 3}, 1},
		{"nested array", []interface{}{[]interface{}{1, 2}}, 2},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := nestingDepth(tt.input); got != tt.want {
				t.Errorf("nestingDepth() = %d, want %d", got, tt.want)
			}
		})
	}
}

func TestJSONEncoder_Encode_EmptyCursor(t *testing.T) {
	registry := newRegistry()
	enc := NewJSONEncoder(registry)

	cursor := &mockCursor{
		batches: [][]Exportable{},
		hasMore: false,
	}

	var buf bytes.Buffer
	if err := enc.Encode(context.Background(), cursor, &buf); err != nil {
		t.Fatalf("Encode() error = %v", err)
	}

	if buf.Len() != 0 {
		t.Errorf("expected empty output, got %q", buf.String())
	}
}

func TestJSONDecoder_Validate_EmptyInput(t *testing.T) {
	dec := NewJSONDecoder(newRegistry())

	preview, err := dec.Validate(context.Background(), strings.NewReader(""))
	if err != nil {
		t.Fatalf("Validate() error = %v", err)
	}

	if preview.TotalRecords != 0 {
		t.Errorf("TotalRecords = %d, want 0", preview.TotalRecords)
	}
}

func TestJSONDecoder_Validate_DepthExceeded(t *testing.T) {
	dec := NewJSONDecoder(newRegistry())

	deepData := `{"a":{"b":{"c":{"d":{"e":{"f":{"g":{"h":{"i":{"j":{"k":"deep"}}}}}}}}}}}`

	input := fmt.Sprintf(`{"type":"users","id":"1","data":{"id":"1","extra":%s}}`, deepData)

	preview, err := dec.Validate(context.Background(), strings.NewReader(input))
	if err == nil {
		t.Error("expected validation error for nesting depth, got nil")
	}

	found := false
	for _, e := range preview.ValidationErrors {
		if strings.Contains(e, "nesting depth exceeds") {
			found = true
			break
		}
	}
	if !found {
		t.Errorf("expected 'nesting depth exceeds' error, got %v", preview.ValidationErrors)
	}
}

func TestJSONDecoder_Decode_SameTypeRepeated(t *testing.T) {
	dec := NewJSONDecoder(newRegistry())

	orgID1 := uuid.New().String()
	orgID2 := uuid.New().String()

	input := fmt.Sprintf(
		`{"type":"organizations","id":"%s","data":{"id":"%s","name":"Org1"}}
{"type":"organizations","id":"%s","data":{"id":"%s","name":"Org2"}}`,
		orgID1, orgID1, orgID2, orgID2,
	)

	ch, err := dec.Decode(context.Background(), strings.NewReader(input))
	if err != nil {
		t.Fatalf("Decode() error = %v", err)
	}

	var records []ImportRecord
	for rec := range ch {
		records = append(records, rec)
	}

	if len(records) != 2 {
		t.Fatalf("expected 2 records, got %d", len(records))
	}

	for _, rec := range records {
		if len(rec.Errors) > 0 {
			t.Errorf("unexpected errors: %v", rec.Errors)
		}
	}
}

func TestJSONDecoder_Decode_BlankLinesSkipped(t *testing.T) {
	dec := NewJSONDecoder(newRegistry())

	orgID := uuid.New().String()

	input := fmt.Sprintf(
		"\n\n{\"type\":\"organizations\",\"id\":\"%s\",\"data\":{\"id\":\"%s\",\"name\":\"Org\"}}\n\n",
		orgID, orgID,
	)

	ch, err := dec.Decode(context.Background(), strings.NewReader(input))
	if err != nil {
		t.Fatalf("Decode() error = %v", err)
	}

	var records []ImportRecord
	for rec := range ch {
		records = append(records, rec)
	}

	if len(records) != 1 {
		t.Fatalf("expected 1 record (blank lines skipped), got %d", len(records))
	}
}