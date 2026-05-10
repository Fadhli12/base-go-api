# FormatEncoder / FormatDecoder Interface Contracts

## FormatEncoder

```go
// FormatEncoder serializes domain entities into a specific output format.
// Implementations: JSONEncoder, CSVEncoder
type FormatEncoder interface {
    // ContentType returns the MIME type for HTTP response headers.
    // JSON: "application/x-ndjson", CSV: "text/csv"
    ContentType() string
    
    // FileExtension returns the file extension without dot.
    // JSON: "json", CSV: "csv"
    FileExtension() string
    
    // Encode reads entities from the cursor and writes them to w in the
    // target format. Must respect context cancellation.
    // For JSON: writes NDJSON (one JSON object per line)
    // For CSV: writes header line + data rows
    Encode(ctx context.Context, cursor ExportCursor, w io.Writer) error
}
```

## FormatDecoder

```go
// FormatDecoder deserializes data from a specific input format into Importable records.
// Implementations: JSONDecoder, CSVDecoder
type FormatDecoder interface {
    // ContentType returns the expected MIME type for validation.
    ContentType() string
    
    // FileExtension returns the expected file extension.
    FileExtension() string
    
    // CanValidate returns true if this format supports metadata-only validation
    // without database access (used for dry-run preview).
    CanValidate() bool
    
    // Validate performs metadata-only validation on the input stream.
    // Checks: format correctness, required fields, row count, nesting depth.
    // Does NOT check database constraints (unique violations, FK existence).
    Validate(ctx context.Context, r io.Reader) (*ImportPreview, error)
    
    // Decode reads the input stream and yields Importable records via a channel.
    // The caller must drain the channel to avoid goroutine leaks.
    // Records are yielded in topological order per entity type.
    Decode(ctx context.Context, r io.Reader) (<-chan Importable, error)
}
```

## ExportCursor

```go
// ExportCursor provides keyset-paginated access to entities for streaming export.
// Uses WHERE id > lastID ORDER BY id LIMIT batchSize for memory-safe iteration.
type ExportCursor interface {
    // Next fetches the next batch of entities. Returns io.EOF when no more records.
    // Each call advances the cursor position. Implementations MUST use keyset
    // pagination, NOT offset-based pagination.
    Next(ctx context.Context, batchSize int) ([]Exportable, error)
    
    // HasMore returns true if more records are available beyond the current cursor.
    HasMore() bool
    
    // Close releases any resources held by the cursor.
    Close() error
}
```

## Exportable / Importable

```go
// Exportable is implemented by domain entities that can be exported.
// Each entity defines its export representation (stripping internal fields,
// applying PII hashing for non-superadmins, etc.)
type Exportable interface {
    // ToExportResponse converts the entity to its export DTO.
    // piiHash indicates whether PII fields should be hashed.
    ToExportResponse(piiHash bool) any
    // GetEntityType returns the entity type identifier (e.g., "users", "roles").
    GetEntityType() string
}

// Importable represents a single record parsed from an import file,
// ready for insertion into the database after UUID mapping.
type Importable struct {
    EntityType  string         // e.g., "users", "roles"
    ExternalID  uuid.UUID      // UUID from the import file
    Data        map[string]any // Parsed field values
    Errors      []string       // Validation errors for this record
}
```

## Format Factory

```go
// FormatFactory creates encoders and decoders by format name.
type FormatFactory struct {
    encoders map[string]FormatEncoder
    decoders map[string]FormatDecoder
}

func NewFormatFactory() *FormatFactory {
    f := &FormatFactory{
        encoders: make(map[string]FormatEncoder),
        decoders: make(map[string]FormatDecoder),
    }
    f.RegisterEncoder("json", NewJSONEncoder())
    f.RegisterEncoder("csv", NewCSVEncoder())
    f.RegisterDecoder("json", NewJSONDecoder())
    f.RegisterDecoder("csv", NewCSVDecoder())
    return f
}

func (f *FormatFactory) GetEncoder(format string) (FormatEncoder, error) { ... }
func (f *FormatFactory) GetDecoder(format string) (FormatDecoder, error) { ... }
func (f *FormatFactory) RegisterEncoder(format string, encoder FormatEncoder) { ... }
func (f *FormatFactory) RegisterDecoder(format string, decoder FormatDecoder) { ... }
```

## Invariants

1. **FormatEncoder.Encode** MUST write to `w` in streaming fashion — it MUST NOT buffer all entities in memory before writing.
2. **FormatDecoder.Decode** MUST yield records via channel — it MUST NOT block the caller after returning the channel.
3. **ExportCursor.Next** MUST use keyset pagination (`WHERE id > lastID`) — offset pagination degrades on large tables.
4. **CanValidate** returns `true` for JSON and CSV (both support metadata validation), `false` for binary formats (future).
5. **FormatFactory** is initialized once at server startup. Adding a new format requires only implementing the interfaces and registering — zero service changes.
6. **PII hashing**: `ToExportResponse(true)` hashes email fields; `ToExportResponse(false)` includes full data (superadmin only).
7. **Blocked entities**: `EntityRegistry.IsExportable(entityType)` MUST be checked before any export/import operation. If false, the operation MUST fail immediately regardless of Casbin permissions.