package request

// UploadVersionRequest represents the upload version request parameters.
// The file is extracted from the multipart form, not from JSON body.
type UploadVersionRequest struct {
	Collection string `form:"collection" validate:"omitempty,max=255"`
}
