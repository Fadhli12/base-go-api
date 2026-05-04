# Open Questions

## notification-system - 2026-05-01

- [ ] Should notification deletion be a hard delete or should we add an `archived_at` column for hiding without losing data? -- Affects audit trail completeness vs. user control over their inbox.
- [ ] What should the default notification preferences be for new users who have no preference records? -- Current plan assumes email enabled for system/assignment/mention, disabled for invoice.created/news.published. Confirm this matches product intent.
- [ ] Should there be a rate limit on notification creation per user (e.g., max 100/hour) to prevent spam from programmatic callers? -- Matters for system integrity if multiple services fire notifications concurrently.
- [ ] Is the notification type list (mention, assignment, system, invoice.created, news.published) final, or should we use a VARCHAR without CHECK constraint to allow runtime extensibility? -- CHECK constraint is safer but requires a migration for each new type.
- [ ] Should the `MarkAllAsRead` endpoint accept an optional `type` filter (e.g., mark all "system" notifications as read)? -- Improves UX for users with many notifications across types.
