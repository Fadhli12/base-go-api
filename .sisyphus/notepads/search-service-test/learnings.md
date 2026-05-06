# Search Service Test Learnings

## Test Pattern
- Each sub-test creates its own mock repository to avoid state leakage
- Service instances created fresh per sub-test to avoid mock state issues

## Common Issues
1. **Shared mocks between sub-tests**: Mock expectations from one sub-test can "leak" to another if they share the same mock instance. Always create fresh mocks inside each subtest.

2. **Private helper functions not exported**: `buildTSQuery`, `appendFilter`, `orderClause` are unexported (lowercase). Cannot test directly from unit test package. Options:
   - Test via exported methods (Search)
   - Move to separate testable package
   - Make functions package-level (but breaks encapsulation)

3. **Query truncation test**: The `maxQueryLength` constant (500) is tested indirectly since `buildTSQuery` itself doesn't truncate - truncation happens in `Search()` before calling the helper.

## What's Tested
- CreateSavedSearch: validation (empty name, long name, empty query), limit check (50), successful creation
- ListSavedSearches: returns user searches, handles repo errors
- GetSavedSearch: ownership check, not found cases
- UpdateSavedSearch: name/query update, ownership check, validation
- DeleteSavedSearch: soft delete, ownership check, error handling
- Whitespace trimming behavior
- Empty value skip behavior in updates
