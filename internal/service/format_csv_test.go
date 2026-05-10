package service

import (
	"bytes"
	"context"
	"encoding/csv"
	"fmt"
	"strings"
	"testing"

	"github.com/google/uuid"
)

func TestCSVEncoder_ContentType(t *testing.T) {
	enc := NewCSVEncoder(newRegistry())
	if got := enc.ContentType(); got != "text/csv" {
		t.Errorf("ContentType() = %q, want %q", got, "text/csv")
	}
}

func TestCSVEncoder_FileExtension(t *testing.T) {
	enc := NewCSVEncoder(newRegistry())
	if got := enc.FileExtension(); got != "csv" {
		t.Errorf("FileExtension() = %q, want %q", got, "csv")
	}
}

func TestCSVEncoder_Encode_WritesHeaderAndRows(t *testing.T) {
	registry := newRegistry()
	enc := NewCSVEncoder(registry)

	orgID := uuid.New().String()
	userID := uuid.New().String()
	entities := []Exportable{
		&mockExportable{entityType: "organizations", data: map[string]interface{}{"id": orgID, "name": "TestOrg"}},
		&mockExportable{entityType: "users", data: map[string]interface{}{"id": userID, "name": "TestUser", "email": "test@example.com"}},
	}
	cursor := &mockCursor{
		batches: [][]Exportable{entities},
		hasMore: true,
	}

	var buf bytes.Buffer
	if err := enc.Encode(context.Background(), cursor, &buf); err != nil {
		t.Fatalf("Encode() error = %v", err)
	}

	reader := csv.NewReader(&buf)
	records, err := reader.ReadAll()
	if err != nil {
		t.Fatalf("failed to parse CSV output: %v", err)
	}

	if len(records) != 3 {
		t.Fatalf("expected 3 rows (1 header + 2 data), got %d", len(records))
	}

	header := records[0]
	if header[0] != "_type" {
		t.Errorf("header first column = %q, want _type", header[0])
	}

	if records[1][0] != "organizations" {
		t.Errorf("row 1 type = %q, want organizations", records[1][0])
	}
	if records[2][0] != "users" {
		t.Errorf("row 2 type = %q, want users", records[2][0])
	}
}

func TestCSVEncoder_Encode_SkipsJunctionEntities(t *testing.T) {
	registry := newRegistry()
	enc := NewCSVEncoder(registry)

	orgID := uuid.New().String()
	entities := []Exportable{
		&mockExportable{entityType: "organizations", data: map[string]interface{}{"id": orgID, "name": "Org"}},
		&mockExportable{entityType: "user_roles", data: map[string]interface{}{"id": uuid.New().String(), "user_id": uuid.New().String(), "role_id": uuid.New().String()}},
		&mockExportable{entityType: "org_members", data: map[string]interface{}{"id": uuid.New().String()}},
		&mockExportable{entityType: "user_permissions", data: map[string]interface{}{"id": uuid.New().String()}},
	}
	cursor := &mockCursor{
		batches: [][]Exportable{entities},
		hasMore: true,
	}

	var buf bytes.Buffer
	if err := enc.Encode(context.Background(), cursor, &buf); err != nil {
		t.Fatalf("Encode() error = %v", err)
	}

	reader := csv.NewReader(&buf)
	records, err := reader.ReadAll()
	if err != nil {
		t.Fatalf("failed to parse CSV: %v", err)
	}

	if len(records) != 2 {
		t.Fatalf("expected 2 rows (header + 1 org), got %d (junction entities should be skipped)", len(records))
	}

	if records[1][0] != "organizations" {
		t.Errorf("expected organizations row, got type=%q", records[1][0])
	}
}

func TestCSVEncoder_Encode_SkipsRestrictedEntities(t *testing.T) {
	registry := newRegistry()
	enc := NewCSVEncoder(registry)

	orgID := uuid.New().String()
	entities := []Exportable{
		&mockExportable{entityType: "api_keys", data: map[string]interface{}{"id": uuid.New().String()}},
		&mockExportable{entityType: "organizations", data: map[string]interface{}{"id": orgID, "name": "Org"}},
	}
	cursor := &mockCursor{
		batches: [][]Exportable{entities},
		hasMore: true,
	}

	var buf bytes.Buffer
	if err := enc.Encode(context.Background(), cursor, &buf); err != nil {
		t.Fatalf("Encode() error = %v", err)
	}

	reader := csv.NewReader(&buf)
	records, err := reader.ReadAll()
	if err != nil {
		t.Fatalf("parse CSV: %v", err)
	}

	if len(records) != 2 {
		t.Fatalf("expected 2 rows (header + 1 org), got %d", len(records))
	}
}

func TestCSVEncoder_Encode_KeysetPagination(t *testing.T) {
	registry := newRegistry()
	enc := NewCSVEncoder(registry)

	batch1 := []Exportable{makeEntity("organizations", uuid.New().String(), nil)}
	batch2 := []Exportable{makeEntity("users", uuid.New().String(), nil)}
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

	reader := csv.NewReader(&buf)
	records, err := reader.ReadAll()
	if err != nil {
		t.Fatalf("parse CSV: %v", err)
	}

	if len(records) != 3 {
		t.Fatalf("expected 3 rows (header + 2 data), got %d", len(records))
	}
}

func TestCSVEncoder_Encode_ContextCancellation(t *testing.T) {
	registry := newRegistry()
	enc := NewCSVEncoder(registry)

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

func TestCSVEncoder_Encode_CursorError(t *testing.T) {
	registry := newRegistry()
	enc := NewCSVEncoder(registry)

	cursor := &mockCursor{
		batches:  [][]Exportable{},
		hasMore:  true,
		nextErr:  fmt.Errorf("db connection lost"),
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

func TestCSVEncoder_Encode_ClosesCursor(t *testing.T) {
	registry := newRegistry()
	enc := NewCSVEncoder(registry)

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

func TestCSVDecoder_ContentType(t *testing.T) {
	dec := NewCSVDecoder(newRegistry())
	if got := dec.ContentType(); got != "text/csv" {
		t.Errorf("ContentType() = %q, want %q", got, "text/csv")
	}
}

func TestCSVDecoder_FileExtension(t *testing.T) {
	dec := NewCSVDecoder(newRegistry())
	if got := dec.FileExtension(); got != "csv" {
		t.Errorf("FileExtension() = %q, want %q", got, "csv")
	}
}

func TestCSVDecoder_CanValidate(t *testing.T) {
	dec := NewCSVDecoder(newRegistry())
	if !dec.CanValidate() {
		t.Error("CanValidate() = false, want true")
	}
}

func TestCSVDecoder_Validate_CorrectCount(t *testing.T) {
	dec := NewCSVDecoder(newRegistry())

	orgID := uuid.New().String()
	userID1 := uuid.New().String()
	userID2 := uuid.New().String()

	input := fmt.Sprintf("_type,id,name\norganizations,%s,Org\nusers,%s,User1\nusers,%s,User2\n",
		orgID, userID1, userID2)

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

func TestCSVDecoder_Validate_RejectsJunctionEntityType(t *testing.T) {
	tests := []struct {
		name        string
		entityType  string
		errorSubstr string
	}{
		{"org_members", "org_members", "not supported in CSV format"},
		{"user_roles", "user_roles", "not supported in CSV format"},
		{"user_permissions", "user_permissions", "not supported in CSV format"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			dec := NewCSVDecoder(newRegistry())
			input := fmt.Sprintf("_type,id,name\n%s,%s,Test\n", tt.entityType, uuid.New().String())

			preview, err := dec.Validate(context.Background(), strings.NewReader(input))
			if err == nil {
				t.Fatal("expected validation error for junction entity, got nil")
			}

			found := false
			for _, e := range preview.ValidationErrors {
				if strings.Contains(e, tt.errorSubstr) {
					found = true
					break
				}
			}
			if !found {
				t.Errorf("expected error containing %q, got %v", tt.errorSubstr, preview.ValidationErrors)
			}
		})
	}
}

func TestCSVDecoder_Validate_RejectsMissingTypeColumn(t *testing.T) {
	dec := NewCSVDecoder(newRegistry())

	input := "id,name\norg1,Org\n"

	preview, err := dec.Validate(context.Background(), strings.NewReader(input))
	if err == nil {
		t.Fatal("expected validation error for missing _type column")
	}

	found := false
	for _, e := range preview.ValidationErrors {
		if strings.Contains(e, "missing required column '_type'") {
			found = true
			break
		}
	}
	if !found {
		t.Errorf("expected '_type' missing error, got %v", preview.ValidationErrors)
	}
}

func TestCSVDecoder_Validate_RejectsMissingIDColumn(t *testing.T) {
	dec := NewCSVDecoder(newRegistry())

	input := "_type,name\norganizations,Org\n"

	preview, err := dec.Validate(context.Background(), strings.NewReader(input))
	if err == nil {
		t.Fatal("expected validation error for missing id column")
	}

	found := false
	for _, e := range preview.ValidationErrors {
		if strings.Contains(e, "missing required column 'id'") {
			found = true
			break
		}
	}
	if !found {
		t.Errorf("expected 'id' missing error, got %v", preview.ValidationErrors)
	}
}

func TestCSVDecoder_Validate_RejectsRestrictedEntityType(t *testing.T) {
	dec := NewCSVDecoder(newRegistry())

	input := fmt.Sprintf("_type,id,name\napi_keys,%s,Key\n", uuid.New().String())

	preview, err := dec.Validate(context.Background(), strings.NewReader(input))
	if err == nil {
		t.Fatal("expected validation error for restricted entity type")
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

func TestCSVDecoder_Validate_RejectsUnknownEntityType(t *testing.T) {
	dec := NewCSVDecoder(newRegistry())

	input := fmt.Sprintf("_type,id,name\nunknown_type,%s,Test\n", uuid.New().String())

	preview, err := dec.Validate(context.Background(), strings.NewReader(input))
	if err == nil {
		t.Fatal("expected validation error for unknown entity type")
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

func TestCSVDecoder_Validate_RejectsExceedsLimit(t *testing.T) {
	dec := NewCSVDecoder(newRegistry())

	var buf bytes.Buffer
	buf.WriteString("_type,id,name\n")
	for i := 0; i < maxImportEntities+1; i++ {
		buf.WriteString(fmt.Sprintf("users,%s,User%d\n", uuid.New().String(), i))
	}

	preview, err := dec.Validate(context.Background(), &buf)
	if err == nil {
		t.Fatal("expected validation error for exceeding limit")
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

func TestCSVDecoder_Validate_EmptyCSV(t *testing.T) {
	dec := NewCSVDecoder(newRegistry())

	preview, err := dec.Validate(context.Background(), strings.NewReader(""))
	if err == nil {
		t.Fatal("expected validation error for empty CSV")
	}

	found := false
	for _, e := range preview.ValidationErrors {
		if strings.Contains(e, "empty CSV") {
			found = true
			break
		}
	}
	if !found {
		t.Errorf("expected 'empty CSV' error, got %v", preview.ValidationErrors)
	}
}

func TestCSVDecoder_Validate_ContextCancellation(t *testing.T) {
	dec := NewCSVDecoder(newRegistry())

	ctx, cancel := context.WithCancel(context.Background())
	cancel()

	input := fmt.Sprintf("_type,id,name\norganizations,%s,Org\n", uuid.New().String())
	_, err := dec.Validate(ctx, strings.NewReader(input))
	if err != context.Canceled {
		t.Errorf("expected context.Canceled, got %v", err)
	}
}

func TestCSVDecoder_Decode_YieldsRecords(t *testing.T) {
	dec := NewCSVDecoder(newRegistry())

	orgID := uuid.New().String()
	userID := uuid.New().String()

	input := fmt.Sprintf("_type,id,name\norganizations,%s,Org\nusers,%s,User\n", orgID, userID)

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

	if records[0].EntityType != "organizations" {
		t.Errorf("record 0 type = %q, want organizations", records[0].EntityType)
	}
	if records[1].EntityType != "users" {
		t.Errorf("record 1 type = %q, want users", records[1].EntityType)
	}

	if records[0].ExternalID == uuid.Nil {
		t.Error("expected non-nil ExternalID for record 0")
	}
}

func TestCSVDecoder_Decode_RejectsJunctionEntity(t *testing.T) {
	dec := NewCSVDecoder(newRegistry())

	input := fmt.Sprintf("_type,id,name\norg_members,%s,Test\n", uuid.New().String())

	ch, err := dec.Decode(context.Background(), strings.NewReader(input))
	if err != nil {
		t.Fatalf("Decode() error = %v", err)
	}

	var records []ImportRecord
	for rec := range ch {
		records = append(records, rec)
	}

	if len(records) != 1 {
		t.Fatalf("expected 1 record (with errors), got %d", len(records))
	}

	if len(records[0].Errors) == 0 {
		t.Error("expected errors for junction entity, got none")
	}
	if !strings.Contains(records[0].Errors[0], "not supported in CSV format") {
		t.Errorf("expected 'not supported in CSV format' error, got %v", records[0].Errors)
	}
}

func TestCSVDecoder_Decode_ContextCancellation(t *testing.T) {
	dec := NewCSVDecoder(newRegistry())

	ctx, cancel := context.WithCancel(context.Background())

	input := fmt.Sprintf("_type,id,name\norganizations,%s,Org\n", uuid.New().String())

	ch, err := dec.Decode(ctx, strings.NewReader(input))
	if err != nil {
		t.Fatalf("Decode() error = %v", err)
	}

	cancel()

	for range ch {
	}
}

func TestCSVRoundTrip(t *testing.T) {
	registry := newRegistry()
	enc := NewCSVEncoder(registry)
	dec := NewCSVDecoder(registry)

	orgID := uuid.New().String()
	roleID := uuid.New().String()
	userID := uuid.New().String()

	entities := []Exportable{
		&mockExportable{entityType: "organizations", data: map[string]interface{}{"id": orgID, "name": "TestOrg"}},
		&mockExportable{entityType: "roles", data: map[string]interface{}{"id": roleID, "name": "Admin"}},
		&mockExportable{entityType: "users", data: map[string]interface{}{"id": userID, "name": "TestUser"}},
	}
	cursor := &mockCursor{
		batches: [][]Exportable{entities},
		hasMore: true,
	}

	var buf bytes.Buffer
	if err := enc.Encode(context.Background(), cursor, &buf); err != nil {
		t.Fatalf("Encode() error = %v", err)
	}

	ch, err := dec.Decode(context.Background(), &buf)
	if err != nil {
		t.Fatalf("Decode() error = %v", err)
	}

	var records []ImportRecord
	for rec := range ch {
		records = append(records, rec)
	}

	if len(records) != 3 {
		t.Fatalf("round-trip: expected 3 records, got %d", len(records))
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

	if records[0].Data["name"] != "TestOrg" {
		t.Errorf("record 0 name = %v, want TestOrg", records[0].Data["name"])
	}
	if records[1].Data["name"] != "Admin" {
		t.Errorf("record 1 name = %v, want Admin", records[1].Data["name"])
	}
	if records[2].Data["name"] != "TestUser" {
		t.Errorf("record 2 name = %v, want TestUser", records[2].Data["name"])
	}
}

func TestCSVEncoder_Encode_MultipleBatches(t *testing.T) {
	registry := newRegistry()
	enc := NewCSVEncoder(registry)

	orgID := uuid.New().String()
	userID := uuid.New().String()

	batch1 := []Exportable{
		&mockExportable{entityType: "organizations", data: map[string]interface{}{"id": orgID, "name": "Org1"}},
	}
	batch2 := []Exportable{
		&mockExportable{entityType: "users", data: map[string]interface{}{"id": userID, "name": "User1", "email": "u@example.com"}},
	}

	cursor := &mockCursor{
		batches: [][]Exportable{batch1, batch2},
		hasMore: true,
	}

	var buf bytes.Buffer
	if err := enc.Encode(context.Background(), cursor, &buf); err != nil {
		t.Fatalf("Encode() error = %v", err)
	}

	reader := csv.NewReader(&buf)
	records, err := reader.ReadAll()
	if err != nil {
		t.Fatalf("parse CSV: %v", err)
	}

	if len(records) != 3 {
		t.Fatalf("expected 3 rows (1 header + 2 data), got %d", len(records))
	}
}

func TestCSVDecoder_Validate_ParsesFieldCount(t *testing.T) {
	dec := NewCSVDecoder(newRegistry())

	orgID := uuid.New().String()
	input := fmt.Sprintf("_type,id,name,email\norganizations,%s,Org,org@test.com\n", orgID)

	preview, err := dec.Validate(context.Background(), strings.NewReader(input))
	if err != nil {
		t.Fatalf("Validate() error = %v", err)
	}

	if preview.TotalRecords != 1 {
		t.Errorf("TotalRecords = %d, want 1", preview.TotalRecords)
	}
	if preview.RecordsByType["organizations"] != 1 {
		t.Errorf("organizations count = %d, want 1", preview.RecordsByType["organizations"])
	}
}

func TestCSVEncoder_Encode_SortedKeys(t *testing.T) {
	registry := newRegistry()
	enc := NewCSVEncoder(registry)

	orgID := uuid.New().String()
	entities := []Exportable{
		&mockExportable{entityType: "organizations", data: map[string]interface{}{
			"id":   orgID,
			"name": "TestOrg",
		}},
	}
	cursor := &mockCursor{
		batches: [][]Exportable{entities},
		hasMore: true,
	}

	var buf bytes.Buffer
	if err := enc.Encode(context.Background(), cursor, &buf); err != nil {
		t.Fatalf("Encode() error = %v", err)
	}

	reader := csv.NewReader(&buf)
	records, err := reader.ReadAll()
	if err != nil {
		t.Fatalf("parse CSV: %v", err)
	}

	if len(records) < 1 {
		t.Fatal("expected at least header row")
	}

	header := records[0]
	if header[0] != "_type" {
		t.Errorf("header[0] = %q, want _type", header[0])
	}

	prev := header[1]
	for i := 2; i < len(header); i++ {
		if header[i] < prev {
			t.Errorf("header keys not sorted: %q comes after %q", header[i], prev)
		}
		prev = header[i]
	}
}

func TestCSVEncoder_Encode_NoData_WritesNoRows(t *testing.T) {
	registry := newRegistry()
	enc := NewCSVEncoder(registry)

	cursor := &mockCursor{
		batches: [][]Exportable{},
		hasMore: false,
	}

	var buf bytes.Buffer
	if err := enc.Encode(context.Background(), cursor, &buf); err != nil {
		t.Fatalf("Encode() error = %v", err)
	}

	if buf.Len() != 0 {
		t.Errorf("expected empty output for no data, got %q", buf.String())
	}
}

func TestCSVDecoder_Decode_MissingTypeColumn(t *testing.T) {
	dec := NewCSVDecoder(newRegistry())

	input := "id,name\norg1,Org\n"

	_, err := dec.Decode(context.Background(), strings.NewReader(input))
	if err == nil {
		t.Fatal("expected error for missing _type column")
	}
	if !strings.Contains(err.Error(), "_type") {
		t.Errorf("expected '_type' error, got %v", err)
	}
}