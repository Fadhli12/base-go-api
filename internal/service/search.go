package service

import (
	"context"
	"fmt"
	"log/slog"
	"regexp"
	"strings"

	"github.com/example/go-api-base/internal/domain"
	"github.com/example/go-api-base/internal/repository"
	apperrors "github.com/example/go-api-base/pkg/errors"
	"github.com/google/uuid"
	"gorm.io/gorm"
)

const (
	maxQueryLength     = 500
	maxSavedSearches   = 50
	defaultPageSize    = 20
	maxPageSize        = 100
	maxSavedSearchName = 255
)

var tsQuerySafeWord = regexp.MustCompile(`[^\w]+`)

// SearchService handles full-text search and saved search operations.
type SearchService struct {
	db              *gorm.DB
	savedSearchRepo repository.SavedSearchRepository
	log             *slog.Logger
}

// NewSearchService creates a new SearchService instance.
func NewSearchService(
	db *gorm.DB,
	savedSearchRepo repository.SavedSearchRepository,
	log *slog.Logger,
) *SearchService {
	return &SearchService{
		db:              db,
		savedSearchRepo: savedSearchRepo,
		log:             log,
	}
}

// Search performs a full-text search on news articles with faceted filtering,
// relevance ranking via ts_rank_cd(), and result highlighting via ts_headline().
//
// Query is sanitised and truncated at 500 characters. Prefix matching is
// enabled by appending :* to each token before passing them to to_tsquery().
func (s *SearchService) Search(
	ctx context.Context,
	userID uuid.UUID,
	query string,
	filters map[string]interface{},
	page, pageSize int,
	sortBy, sortDir string,
) (*repository.SearchResult[map[string]interface{}], error) {

	if page < 1 {
		page = 1
	}
	if pageSize < 1 || pageSize > maxPageSize {
		pageSize = defaultPageSize
	}

	query = strings.TrimSpace(query)
	if len(query) > maxQueryLength {
		query = query[:maxQueryLength]
	}

	tsq := buildTSQuery(query)

	parts := []string{"n.deleted_at IS NULL"}
	args := []interface{}{}

	if query != "" && tsq != "" {
		parts = append(parts, "n.search_vector @@ to_tsquery('english', ?)")
		args = append(args, tsq)
	}

	parts, args = appendFilter(parts, args, "n.status = ?", filters, "status")
	parts, args = appendFilter(parts, args, "n.author_id = ?", filters, "author_id")
	parts, args = appendFilter(parts, args, "n.created_at >= ?::timestamptz", filters, "date_from")
	parts, args = appendFilter(parts, args, "n.created_at <= ?::timestamptz", filters, "date_to")

	whereClause := strings.Join(parts, " AND ")

	var total int64
	if err := s.db.WithContext(ctx).
		Table("news n").
		Where(whereClause, args...).
		Count(&total).Error; err != nil {
		s.logSearchError(ctx, userID, query, "count_total", err)
		return nil, apperrors.WrapInternal(err)
	}

	cols := []string{
		"n.id", "n.title", "n.status", "n.author_id",
		"n.excerpt", "n.tags", "n.metadata",
		"n.published_at", "n.created_at", "n.updated_at",
	}

	if query != "" && tsq != "" {
		cols = append(cols,
			`ts_rank_cd(n.search_vector, to_tsquery('english', ?)) AS rank`,
			`ts_headline('english', n.content, to_tsquery('english', ?),
				'MaxWords=50, MinWords=20, ShortWord=3, MaxFragments=3,
				 FragmentDelimiter=...') AS headline`,
		)
	}

	order := orderClause(query, tsq, sortBy, sortDir)

	offset := (page - 1) * pageSize
	// Build execArgs in the order ? appears in the final SQL:
	// SELECT columns come before WHERE, so tsq args for rank/headline come first,
	// then WHERE clause args, then LIMIT/OFFSET.
	var execArgs []interface{}
	if query != "" && tsq != "" {
		// rank and headline to_tsquery placeholders (in SELECT clause)
		execArgs = append(execArgs, tsq, tsq)
	}
	execArgs = append(execArgs, args...) // WHERE clause args (including tsq for search_vector match)
	execArgs = append(execArgs, pageSize, offset)

	selectSQL := strings.Join(cols, ", ")
	sql := fmt.Sprintf(
		"SELECT %s FROM news n WHERE %s ORDER BY %s LIMIT ? OFFSET ?",
		selectSQL, whereClause, order,
	)

	var rawRows []map[string]interface{}
	if err := s.db.WithContext(ctx).Raw(sql, execArgs...).Scan(&rawRows).Error; err != nil {
		s.logSearchError(ctx, userID, query, "execute", err)
		return nil, apperrors.WrapInternal(err)
	}

	highlights := make(map[string][]string)
	for i, row := range rawRows {
		if hl, ok := row["headline"]; ok && hl != nil {
			if s, ok := hl.(string); ok && s != "" {
				highlights[fmt.Sprintf("%d", i)] = []string{s}
			}
		}
		delete(row, "headline")
	}

	s.log.Info("search executed",
		slog.String("user_id", userID.String()),
		slog.String("query", query),
		slog.Int64("result_count", total),
		slog.Int("page", page),
		slog.Int("page_size", pageSize),
	)

	return &repository.SearchResult[map[string]interface{}]{
		Items:      rawRows,
		Total:      total,
		Page:       page,
		PageSize:   pageSize,
		Highlights: highlights,
	}, nil
}

// CreateSavedSearch persists a new saved search for the given user.
func (s *SearchService) CreateSavedSearch(
	ctx context.Context,
	userID uuid.UUID,
	name, queryText string,
	filters map[string]interface{},
) (*domain.SavedSearch, error) {
	name = strings.TrimSpace(name)
	if name == "" || len(name) > maxSavedSearchName {
		return nil, apperrors.NewAppError("VALIDATION_ERROR",
			fmt.Sprintf("name is required and must be under %d characters", maxSavedSearchName), 422)
	}
	if strings.TrimSpace(queryText) == "" {
		return nil, apperrors.NewAppError("VALIDATION_ERROR", "query_text is required", 422)
	}

	existing, err := s.savedSearchRepo.FindByUserID(ctx, userID)
	if err != nil {
		s.log.Error("failed to check saved search limit",
			slog.String("error", err.Error()),
			slog.String("user_id", userID.String()),
		)
		return nil, apperrors.WrapInternal(err)
	}
	if len(existing) >= maxSavedSearches {
		return nil, apperrors.NewAppError("LIMIT_EXCEEDED",
			fmt.Sprintf("maximum of %d saved searches reached", maxSavedSearches), 422)
	}

	var filtersJSON []byte
	if filters != nil {
		filtersJSON, err = domain.NewJSONB(filters)
		if err != nil {
			return nil, apperrors.NewAppError("VALIDATION_ERROR", "invalid filters format", 422)
		}
	} else {
		filtersJSON = []byte("{}")
	}

	ss := &domain.SavedSearch{
		UserID:    userID,
		Name:      name,
		QueryText: queryText,
		Filters:   filtersJSON,
	}

	if err := s.savedSearchRepo.Create(ctx, ss); err != nil {
		s.log.Error("failed to create saved search",
			slog.String("error", err.Error()),
			slog.String("user_id", userID.String()),
		)
		return nil, apperrors.WrapInternal(err)
	}

	s.log.Info("saved search created",
		slog.String("id", ss.ID.String()),
		slog.String("user_id", userID.String()),
		slog.String("name", name),
	)

	return ss, nil
}

// ListSavedSearches returns all saved searches belonging to a user.
func (s *SearchService) ListSavedSearches(
	ctx context.Context,
	userID uuid.UUID,
) ([]domain.SavedSearch, error) {
	searches, err := s.savedSearchRepo.FindByUserID(ctx, userID)
	if err != nil {
		s.log.Error("failed to list saved searches",
			slog.String("error", err.Error()),
			slog.String("user_id", userID.String()),
		)
		return nil, apperrors.WrapInternal(err)
	}
	return searches, nil
}

// GetSavedSearch retrieves a single saved search, enforcing ownership.
func (s *SearchService) GetSavedSearch(
	ctx context.Context,
	userID, searchID uuid.UUID,
) (*domain.SavedSearch, error) {
	ss, err := s.savedSearchRepo.FindByID(ctx, searchID)
	if err != nil {
		return nil, err
	}
	if ss.UserID != userID {
		return nil, apperrors.ErrNotFound
	}
	return ss, nil
}

// UpdateSavedSearch updates name and/or query_text on an existing saved search.
func (s *SearchService) UpdateSavedSearch(
	ctx context.Context,
	userID, searchID uuid.UUID,
	name, queryText string,
	filters map[string]interface{},
) (*domain.SavedSearch, error) {
	ss, err := s.savedSearchRepo.FindByID(ctx, searchID)
	if err != nil {
		return nil, err
	}
	if ss.UserID != userID {
		return nil, apperrors.ErrNotFound
	}

	if strings.TrimSpace(name) != "" {
		if len(name) > maxSavedSearchName {
			return nil, apperrors.NewAppError("VALIDATION_ERROR",
				fmt.Sprintf("name must be under %d characters", maxSavedSearchName), 422)
		}
		ss.Name = name
	}
	if strings.TrimSpace(queryText) != "" {
		ss.QueryText = queryText
	}
	if filters != nil {
		fjson, err := domain.NewJSONB(filters)
		if err != nil {
			return nil, apperrors.NewAppError("VALIDATION_ERROR", "invalid filters format", 422)
		}
		ss.Filters = fjson
	}

	if err := s.savedSearchRepo.Update(ctx, ss); err != nil {
		s.log.Error("failed to update saved search",
			slog.String("error", err.Error()),
			slog.String("id", searchID.String()),
			slog.String("user_id", userID.String()),
		)
		return nil, apperrors.WrapInternal(err)
	}

	s.log.Info("saved search updated",
		slog.String("id", searchID.String()),
		slog.String("user_id", userID.String()),
	)

	return ss, nil
}

// DeleteSavedSearch soft-deletes a saved search after verifying ownership.
func (s *SearchService) DeleteSavedSearch(
	ctx context.Context,
	userID, searchID uuid.UUID,
) error {
	ss, err := s.savedSearchRepo.FindByID(ctx, searchID)
	if err != nil {
		return err
	}
	if ss.UserID != userID {
		return apperrors.ErrNotFound
	}

	if err := s.savedSearchRepo.SoftDelete(ctx, searchID); err != nil {
		s.log.Error("failed to delete saved search",
			slog.String("error", err.Error()),
			slog.String("id", searchID.String()),
			slog.String("user_id", userID.String()),
		)
		return apperrors.WrapInternal(err)
	}

	s.log.Info("saved search deleted",
		slog.String("id", searchID.String()),
		slog.String("user_id", userID.String()),
	)

	return nil
}

func buildTSQuery(raw string) string {
	raw = strings.TrimSpace(raw)
	if raw == "" {
		return ""
	}

	tokens := strings.Fields(raw)
	safe := make([]string, 0, len(tokens))
	for _, t := range tokens {
		t = strings.ToLower(tsQuerySafeWord.ReplaceAllString(t, ""))
		if t == "" {
			continue
		}
		safe = append(safe, t+":*")
	}
	if len(safe) == 0 {
		return ""
	}
	return strings.Join(safe, " & ")
}

// appendFilter conditionally appends a WHERE clause fragment and its argument.
func appendFilter(parts []string, args []interface{}, clause string,
	filters map[string]interface{}, key string) ([]string, []interface{}) {
	if filters == nil {
		return parts, args
	}
	v, ok := filters[key]
	if !ok {
		return parts, args
	}
	switch val := v.(type) {
	case string:
		if val == "" {
			return parts, args
		}
	case nil:
		return parts, args
	}
	parts = append(parts, clause)
	return parts, append(args, v)
}

// orderClause returns a safe ORDER BY fragment.
func orderClause(query, tsq, sortBy, sortDir string) string {
	if sortDir != "asc" && sortDir != "ASC" {
		sortDir = "DESC"
	} else {
		sortDir = "ASC"
	}

	if query != "" && tsq != "" && (sortBy == "" || sortBy == "relevance") {
		return "rank DESC"
	}

	switch sortBy {
	case "title":
		return "n.title " + sortDir + ", n.created_at DESC"
	case "created_at":
		return "n.created_at " + sortDir
	case "updated_at":
		return "n.updated_at " + sortDir
	default:
		return "n.created_at DESC"
	}
}

// logSearchError is a convenience helper for logging search-related errors.
func (s *SearchService) logSearchError(ctx context.Context, userID uuid.UUID,
	query, stage string, err error) {
	s.log.Error("search failed",
		slog.String("stage", stage),
		slog.String("error", err.Error()),
		slog.String("user_id", userID.String()),
		slog.String("query", query),
	)
}
