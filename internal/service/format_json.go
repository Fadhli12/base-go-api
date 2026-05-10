package service

import (
	"bufio"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"strings"

	"github.com/example/go-api-base/internal/domain"
	"github.com/example/go-api-base/pkg/errors"
	"github.com/google/uuid"
)

const (
	maxImportEntities = 10000
	maxNestingDepth   = 10
	encoderBatchSize  = 100
	decoderBufferSize = 64 * 1024
)

type Exportable interface {
	ToExportRecord() map[string]interface{}
	GetEntityType() string
}

type ExportCursor interface {
	Next(ctx context.Context, batchSize int) ([]Exportable, error)
	HasMore() bool
	Close() error
}

type ImportRecord struct {
	EntityType string
	ExternalID uuid.UUID
	Data       map[string]interface{}
	Errors     []string
}

type ImportPreview struct {
	TotalRecords    int            `json:"total_records"`
	RecordsByType   map[string]int `json:"records_by_type"`
	ValidationErrors []string      `json:"validation_errors"`
	Warnings        []string      `json:"warnings"`
}

type FormatEncoder interface {
	ContentType() string
	FileExtension() string
	Encode(ctx context.Context, cursor ExportCursor, w io.Writer) error
}

type FormatDecoder interface {
	ContentType() string
	FileExtension() string
	CanValidate() bool
	Validate(ctx context.Context, r io.Reader) (*ImportPreview, error)
	Decode(ctx context.Context, r io.Reader) (<-chan ImportRecord, error)
}

type ndjsonRecord struct {
	Type string                 `json:"type"`
	ID   string                 `json:"id"`
	Data map[string]interface{} `json:"data"`
}

type JSONEncoder struct {
	registry *domain.EntityRegistry
}

func NewJSONEncoder(registry *domain.EntityRegistry) *JSONEncoder {
	return &JSONEncoder{registry: registry}
}

func (e *JSONEncoder) ContentType() string {
	return "application/x-ndjson"
}

func (e *JSONEncoder) FileExtension() string {
	return "json"
}

func (e *JSONEncoder) Encode(ctx context.Context, cursor ExportCursor, w io.Writer) error {
	defer func() { _ = cursor.Close() }()

	writer := bufio.NewWriterSize(w, decoderBufferSize)
	defer func() { _ = writer.Flush() }()

	for cursor.HasMore() {
		select {
		case <-ctx.Done():
			return ctx.Err()
		default:
		}

		batch, err := cursor.Next(ctx, encoderBatchSize)
		if err != nil {
			return fmt.Errorf("cursor next: %w", err)
		}
		if len(batch) == 0 {
			break
		}

		for _, entity := range batch {
			select {
			case <-ctx.Done():
				return ctx.Err()
			default:
			}

			if !e.registry.IsExportable(entity.GetEntityType()) {
				continue
			}

			record := ndjsonRecord{
				Type: entity.GetEntityType(),
				Data: entity.ToExportRecord(),
			}
			if id, ok := record.Data["id"]; ok {
				if idStr, ok := id.(string); ok {
					record.ID = idStr
				}
			}

			data, err := json.Marshal(record)
			if err != nil {
				return fmt.Errorf("marshal entity %s: %w", entity.GetEntityType(), err)
			}

			if _, err := writer.Write(data); err != nil {
				return fmt.Errorf("write: %w", err)
			}
			if err := writer.WriteByte('\n'); err != nil {
				return fmt.Errorf("write newline: %w", err)
			}
		}
	}

	return nil
}

type JSONDecoder struct {
	registry *domain.EntityRegistry
}

func NewJSONDecoder(registry *domain.EntityRegistry) *JSONDecoder {
	return &JSONDecoder{registry: registry}
}

func (d *JSONDecoder) ContentType() string {
	return "application/x-ndjson"
}

func (d *JSONDecoder) FileExtension() string {
	return "json"
}

func (d *JSONDecoder) CanValidate() bool {
	return true
}

func (d *JSONDecoder) Validate(ctx context.Context, r io.Reader) (*ImportPreview, error) {
	preview := &ImportPreview{
		RecordsByType: make(map[string]int),
	}

	scanner := bufio.NewScanner(r)
	scanner.Buffer(make([]byte, 0, decoderBufferSize), decoderBufferSize*16)

	lineNum := 0
	for scanner.Scan() {
		lineNum++
		select {
		case <-ctx.Done():
			return nil, ctx.Err()
		default:
		}

		line := strings.TrimSpace(scanner.Text())
		if line == "" {
			continue
		}

		var record ndjsonRecord
		dec := json.NewDecoder(strings.NewReader(line))
		dec.DisallowUnknownFields()
		if err := dec.Decode(&record); err != nil {
			preview.ValidationErrors = append(preview.ValidationErrors,
				fmt.Sprintf("line %d: invalid JSON: %v", lineNum, err))
			continue
		}

		if record.Type == "" {
			preview.ValidationErrors = append(preview.ValidationErrors,
				fmt.Sprintf("line %d: missing required field 'type'", lineNum))
			continue
		}

		if record.Data == nil {
			preview.ValidationErrors = append(preview.ValidationErrors,
				fmt.Sprintf("line %d: missing required field 'data'", lineNum))
			continue
		}

		if _, ok := record.Data["id"]; !ok {
			preview.ValidationErrors = append(preview.ValidationErrors,
				fmt.Sprintf("line %d: missing required field 'id' in data", lineNum))
			continue
		}

		if d.registry.IsRestricted(record.Type) {
			preview.ValidationErrors = append(preview.ValidationErrors,
				fmt.Sprintf("line %d: entity type '%s' is restricted", lineNum, record.Type))
			continue
		}

		if !d.registry.IsImportable(record.Type) {
			preview.ValidationErrors = append(preview.ValidationErrors,
				fmt.Sprintf("line %d: unknown entity type '%s'", lineNum, record.Type))
			continue
		}

		if nestingDepth(record.Data) > maxNestingDepth {
			preview.ValidationErrors = append(preview.ValidationErrors,
				fmt.Sprintf("line %d: nesting depth exceeds %d", lineNum, maxNestingDepth))
			continue
		}

		preview.RecordsByType[record.Type]++
		preview.TotalRecords++
	}

	if err := scanner.Err(); err != nil {
		preview.ValidationErrors = append(preview.ValidationErrors,
			fmt.Sprintf("read error: %v", err))
	}

	if preview.TotalRecords > maxImportEntities {
		preview.ValidationErrors = append(preview.ValidationErrors,
			fmt.Sprintf("total records %d exceeds limit of %d", preview.TotalRecords, maxImportEntities))
	}

	if len(preview.ValidationErrors) > 0 {
		return preview, errors.NewAppError("VALIDATION_ERROR", "import validation failed", 400)
	}

	return preview, nil
}

func (d *JSONDecoder) Decode(ctx context.Context, r io.Reader) (<-chan ImportRecord, error) {
	ch := make(chan ImportRecord, 256)

	go func() {
		defer close(ch)

		scanner := bufio.NewScanner(r)
		scanner.Buffer(make([]byte, 0, decoderBufferSize), decoderBufferSize*16)

		importOrder := d.registry.GetImportOrder()
		typeOrderIndex := make(map[string]int, len(importOrder))
		for i, t := range importOrder {
			typeOrderIndex[t] = i
		}

		lastTypeOrder := -1
		lineNum := 0

		for scanner.Scan() {
			lineNum++
			select {
			case <-ctx.Done():
				return
			default:
			}

			line := strings.TrimSpace(scanner.Text())
			if line == "" {
				continue
			}

			var record ndjsonRecord
			if err := json.Unmarshal([]byte(line), &record); err != nil {
				ch <- ImportRecord{
					Errors: []string{fmt.Sprintf("line %d: invalid JSON: %v", lineNum, err)},
				}
				continue
			}

			if record.Type == "" {
				ch <- ImportRecord{
					Errors: []string{fmt.Sprintf("line %d: missing type field", lineNum)},
				}
				continue
			}

			if d.registry.IsRestricted(record.Type) {
				ch <- ImportRecord{
					EntityType: record.Type,
					Data:       record.Data,
					Errors:     []string{fmt.Sprintf("line %d: entity type '%s' is restricted", lineNum, record.Type)},
				}
				continue
			}

			if !d.registry.IsImportable(record.Type) {
				ch <- ImportRecord{
					EntityType: record.Type,
					Data:       record.Data,
					Errors:     []string{fmt.Sprintf("line %d: unknown entity type '%s'", lineNum, record.Type)},
				}
				continue
			}

			currentOrder, known := typeOrderIndex[record.Type]
			if known && currentOrder < lastTypeOrder {
				ch <- ImportRecord{
					EntityType: record.Type,
					Data:       record.Data,
					Errors:     []string{fmt.Sprintf("line %d: entity type '%s' out of topological order", lineNum, record.Type)},
				}
				continue
			}
			if known {
				lastTypeOrder = currentOrder
			}

			externalID := uuid.Nil
			if idStr, ok := record.Data["id"]; ok {
				if s, ok := idStr.(string); ok {
					if parsed, err := uuid.Parse(s); err == nil {
						externalID = parsed
					}
				}
			}

			ch <- ImportRecord{
				EntityType: record.Type,
				ExternalID: externalID,
				Data:       record.Data,
			}
		}
	}()

	return ch, nil
}

func nestingDepth(v interface{}) int {
	switch val := v.(type) {
	case map[string]interface{}:
		maxChild := 0
		for _, child := range val {
			d := nestingDepth(child)
			if d > maxChild {
				maxChild = d
			}
		}
		return 1 + maxChild
	case []interface{}:
		maxChild := 0
		for _, child := range val {
			d := nestingDepth(child)
			if d > maxChild {
				maxChild = d
			}
		}
		return 1 + maxChild
	default:
		return 0
	}
}