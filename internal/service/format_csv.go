package service

import (
	"context"
	"encoding/csv"
	"fmt"
	"io"
	"sort"

	"github.com/example/go-api-base/internal/domain"
	"github.com/example/go-api-base/pkg/errors"
	"github.com/google/uuid"
)

var csvJunctionEntities = map[string]bool{
	domain.EntityTypeOrgMember:      true,
	domain.EntityTypeUserRole:       true,
	domain.EntityTypeUserPermission: true,
}

type CSVEncoder struct {
	registry *domain.EntityRegistry
}

func NewCSVEncoder(registry *domain.EntityRegistry) *CSVEncoder {
	return &CSVEncoder{registry: registry}
}

func (e *CSVEncoder) ContentType() string {
	return "text/csv"
}

func (e *CSVEncoder) FileExtension() string {
	return "csv"
}

func (e *CSVEncoder) Encode(ctx context.Context, cursor ExportCursor, w io.Writer) error {
	defer func() { _ = cursor.Close() }()

	csvWriter := csv.NewWriter(w)
	defer csvWriter.Flush()

	var allKeys []string
	keysSeen := make(map[string]bool)

	var entities []Exportable

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

			if csvJunctionEntities[entity.GetEntityType()] {
				continue
			}

			record := entity.ToExportRecord()
			for k := range record {
				if !keysSeen[k] {
					keysSeen[k] = true
					allKeys = append(allKeys, k)
				}
			}
			entities = append(entities, entity)
		}
	}

	sort.Strings(allKeys)

	if len(entities) == 0 {
		return nil
	}

	header := make([]string, 0, len(allKeys)+1)
	header = append(header, "_type")
	header = append(header, allKeys...)
	if err := csvWriter.Write(header); err != nil {
		return fmt.Errorf("write header: %w", err)
	}

	for _, entity := range entities {
		record := entity.ToExportRecord()

		row := make([]string, 0, len(allKeys)+1)
		row = append(row, entity.GetEntityType())
		for _, k := range allKeys {
			v, ok := record[k]
			if !ok {
				row = append(row, "")
			} else {
				row = append(row, formatCSVValue(v))
			}
		}
		if err := csvWriter.Write(row); err != nil {
			return fmt.Errorf("write row: %w", err)
		}
	}

	csvWriter.Flush()
	if err := csvWriter.Error(); err != nil {
		return fmt.Errorf("flush csv: %w", err)
	}

	return nil
}

type CSVDecoder struct {
	registry *domain.EntityRegistry
}

func NewCSVDecoder(registry *domain.EntityRegistry) *CSVDecoder {
	return &CSVDecoder{registry: registry}
}

func (d *CSVDecoder) ContentType() string {
	return "text/csv"
}

func (d *CSVDecoder) FileExtension() string {
	return "csv"
}

func (d *CSVDecoder) CanValidate() bool {
	return true
}

func (d *CSVDecoder) Validate(ctx context.Context, r io.Reader) (*ImportPreview, error) {
	preview := &ImportPreview{
		RecordsByType:   make(map[string]int),
		ValidationErrors: []string{},
		Warnings:        []string{},
	}

	csvReader := csv.NewReader(r)
	csvReader.FieldsPerRecord = -1

	headers, err := csvReader.Read()
	if err != nil {
		if err == io.EOF {
			preview.ValidationErrors = append(preview.ValidationErrors, "empty CSV: no header row")
			return preview, errors.NewAppError("VALIDATION_ERROR", "import validation failed", 400)
		}
		preview.ValidationErrors = append(preview.ValidationErrors, fmt.Sprintf("read header: %v", err))
		return preview, errors.NewAppError("VALIDATION_ERROR", "import validation failed", 400)
	}

	if len(headers) == 0 {
		preview.ValidationErrors = append(preview.ValidationErrors, "CSV header is empty")
		return preview, errors.NewAppError("VALIDATION_ERROR", "import validation failed", 400)
	}

	typeColIdx := -1
	for i, h := range headers {
		if h == "_type" {
			typeColIdx = i
			break
		}
	}
	if typeColIdx == -1 {
		preview.ValidationErrors = append(preview.ValidationErrors, "CSV header missing required column '_type'")
		return preview, errors.NewAppError("VALIDATION_ERROR", "import validation failed", 400)
	}

	idColIdx := -1
	for i, h := range headers {
		if h == "id" {
			idColIdx = i
			break
		}
	}
	if idColIdx == -1 {
		preview.ValidationErrors = append(preview.ValidationErrors, "CSV header missing required column 'id'")
		return preview, errors.NewAppError("VALIDATION_ERROR", "import validation failed", 400)
	}

	rowNum := 1
	for {
		select {
		case <-ctx.Done():
			return nil, ctx.Err()
		default:
		}

		row, err := csvReader.Read()
		if err == io.EOF {
			break
		}
		if err != nil {
			preview.ValidationErrors = append(preview.ValidationErrors,
				fmt.Sprintf("row %d: parse error: %v", rowNum, err))
			continue
		}
		rowNum++

		if typeColIdx >= len(row) {
			preview.ValidationErrors = append(preview.ValidationErrors,
				fmt.Sprintf("row %d: missing _type column", rowNum))
			continue
		}

		entityType := row[typeColIdx]

		if csvJunctionEntities[entityType] {
			preview.ValidationErrors = append(preview.ValidationErrors,
				fmt.Sprintf("row %d: entity type '%s' is not supported in CSV format (junction entity)", rowNum, entityType))
			continue
		}

		if d.registry.IsRestricted(entityType) {
			preview.ValidationErrors = append(preview.ValidationErrors,
				fmt.Sprintf("row %d: entity type '%s' is restricted", rowNum, entityType))
			continue
		}

		if !d.registry.IsImportable(entityType) {
			preview.ValidationErrors = append(preview.ValidationErrors,
				fmt.Sprintf("row %d: unknown entity type '%s'", rowNum, entityType))
			continue
		}

		preview.RecordsByType[entityType]++
		preview.TotalRecords++
	}

	if preview.TotalRecords > maxImportEntities {
		preview.ValidationErrors = append(preview.ValidationErrors,
			fmt.Sprintf("total records %d exceeds limit of %d", preview.TotalRecords, maxImportEntities))
	}

	for junctionType := range csvJunctionEntities {
		if cnt, ok := preview.RecordsByType[junctionType]; ok && cnt > 0 {
			preview.Warnings = append(preview.Warnings,
				fmt.Sprintf("CSV format does not support junction entity '%s'; these records will be skipped", junctionType))
		}
	}

	if len(preview.ValidationErrors) > 0 {
		return preview, errors.NewAppError("VALIDATION_ERROR", "import validation failed", 400)
	}

	return preview, nil
}

func (d *CSVDecoder) Decode(ctx context.Context, r io.Reader) (<-chan ImportRecord, error) {
	csvReader := csv.NewReader(r)
	csvReader.FieldsPerRecord = -1

	headers, err := csvReader.Read()
	if err != nil {
		return nil, fmt.Errorf("read CSV header: %w", err)
	}

	typeColIdx := -1
	for i, h := range headers {
		if h == "_type" {
			typeColIdx = i
			break
		}
	}
	if typeColIdx == -1 {
		return nil, fmt.Errorf("CSV header missing required column '_type'")
	}

	ch := make(chan ImportRecord, 256)

	go func() {
		defer close(ch)

		rowNum := 1
		for {
			select {
			case <-ctx.Done():
				return
			default:
			}

			row, err := csvReader.Read()
			if err == io.EOF {
				break
			}
			if err != nil {
				ch <- ImportRecord{
					Errors: []string{fmt.Sprintf("row %d: parse error: %v", rowNum, err)},
				}
				continue
			}
			rowNum++

			if typeColIdx >= len(row) {
				ch <- ImportRecord{
					Errors: []string{fmt.Sprintf("row %d: missing _type column", rowNum)},
				}
				continue
			}

			entityType := row[typeColIdx]

			if csvJunctionEntities[entityType] {
				ch <- ImportRecord{
					EntityType: entityType,
					Errors:     []string{fmt.Sprintf("row %d: entity type '%s' is not supported in CSV format (junction entity)", rowNum, entityType)},
				}
				continue
			}

			if d.registry.IsRestricted(entityType) {
				ch <- ImportRecord{
					EntityType: entityType,
					Errors:     []string{fmt.Sprintf("row %d: entity type '%s' is restricted", rowNum, entityType)},
				}
				continue
			}

			if !d.registry.IsImportable(entityType) {
				ch <- ImportRecord{
					EntityType: entityType,
					Errors:     []string{fmt.Sprintf("row %d: unknown entity type '%s'", rowNum, entityType)},
				}
				continue
			}

			data := make(map[string]interface{}, len(headers))
			for i, h := range headers {
				if h == "_type" {
					continue
				}
				if i < len(row) {
					data[h] = row[i]
				}
			}

			externalID := uuid.Nil
			if idStr, ok := data["id"]; ok {
				if s, ok := idStr.(string); ok {
					if parsed, err := uuid.Parse(s); err == nil {
						externalID = parsed
					}
				}
			}

			ch <- ImportRecord{
				EntityType: entityType,
				ExternalID: externalID,
				Data:       data,
			}
		}
	}()

	return ch, nil
}

func sortedKeys(m map[string]interface{}) []string {
	keys := make([]string, 0, len(m))
	for k := range m {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	return keys
}

func formatCSVValue(v interface{}) string {
	if v == nil {
		return ""
	}
	switch val := v.(type) {
	case string:
		return val
	case fmt.Stringer:
		return val.String()
	default:
		return fmt.Sprintf("%v", val)
	}
}

