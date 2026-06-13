package repository

import (
	"context"
	"database/sql"
	"sort"
	"strings"
	"time"

	dbent "github.com/Wei-Shaw/sub2api/ent"
	"github.com/Wei-Shaw/sub2api/ent/proxy"
	"github.com/Wei-Shaw/sub2api/internal/pkg/logger"
	"github.com/Wei-Shaw/sub2api/internal/service"

	"github.com/Wei-Shaw/sub2api/internal/pkg/pagination"

	entsql "entgo.io/ent/dialect/sql"
)

type proxyRepository struct {
	client *dbent.Client
	sql    sqlExecutor
}

func NewProxyRepository(client *dbent.Client, sqlDB *sql.DB) service.ProxyRepository {
	return newProxyRepositoryWithSQL(client, sqlDB)
}

func newProxyRepositoryWithSQL(client *dbent.Client, sqlq sqlExecutor) *proxyRepository {
	return &proxyRepository{client: client, sql: sqlq}
}

func (r *proxyRepository) Create(ctx context.Context, proxyIn *service.Proxy) error {
	builder := r.client.Proxy.Create().
		SetName(proxyIn.Name).
		SetProtocol(proxyIn.Protocol).
		SetHost(proxyIn.Host).
		SetPort(proxyIn.Port).
		SetStatus(proxyIn.Status)
	if proxyIn.Username != "" {
		builder.SetUsername(proxyIn.Username)
	}
	if proxyIn.Password != "" {
		builder.SetPassword(proxyIn.Password)
	}

	created, err := builder.Save(ctx)
	if err == nil {
		applyProxyEntityToService(proxyIn, created)
		if saveErr := r.saveProxyExtraFields(ctx, proxyIn); saveErr != nil {
			return saveErr
		}
	}
	return err
}

func (r *proxyRepository) GetByID(ctx context.Context, id int64) (*service.Proxy, error) {
	m, err := r.client.Proxy.Get(ctx, id)
	if err != nil {
		if dbent.IsNotFound(err) {
			return nil, service.ErrProxyNotFound
		}
		return nil, err
	}
	out := proxyEntityToService(m)
	if err := r.loadProxyExtraFields(ctx, out); err != nil {
		return nil, err
	}
	return out, nil
}

func (r *proxyRepository) ListByIDs(ctx context.Context, ids []int64) ([]service.Proxy, error) {
	if len(ids) == 0 {
		return []service.Proxy{}, nil
	}

	proxies, err := r.client.Proxy.Query().
		Where(proxy.IDIn(ids...)).
		All(ctx)
	if err != nil {
		return nil, err
	}

	out := make([]service.Proxy, 0, len(proxies))
	for i := range proxies {
		item := proxyEntityToService(proxies[i])
		if err := r.loadProxyExtraFields(ctx, item); err != nil {
			return nil, err
		}
		out = append(out, *item)
	}
	return out, nil
}

func (r *proxyRepository) Update(ctx context.Context, proxyIn *service.Proxy) error {
	builder := r.client.Proxy.UpdateOneID(proxyIn.ID).
		SetName(proxyIn.Name).
		SetProtocol(proxyIn.Protocol).
		SetHost(proxyIn.Host).
		SetPort(proxyIn.Port).
		SetStatus(proxyIn.Status)
	if proxyIn.Username != "" {
		builder.SetUsername(proxyIn.Username)
	} else {
		builder.ClearUsername()
	}
	if proxyIn.Password != "" {
		builder.SetPassword(proxyIn.Password)
	} else {
		builder.ClearPassword()
	}

	updated, err := builder.Save(ctx)
	if err == nil {
		applyProxyEntityToService(proxyIn, updated)
		if saveErr := r.saveProxyExtraFields(ctx, proxyIn); saveErr != nil {
			return saveErr
		}
		return nil
	}
	if dbent.IsNotFound(err) {
		return service.ErrProxyNotFound
	}
	return err
}

func (r *proxyRepository) Delete(ctx context.Context, id int64) error {
	_, err := r.client.Proxy.Delete().Where(proxy.IDEQ(id)).Exec(ctx)
	return err
}

func (r *proxyRepository) List(ctx context.Context, params pagination.PaginationParams) ([]service.Proxy, *pagination.PaginationResult, error) {
	return r.ListWithFilters(ctx, params, "", "", "")
}

// ListWithFilters lists proxies with optional filtering by protocol, status, and search query
func (r *proxyRepository) ListWithFilters(ctx context.Context, params pagination.PaginationParams, protocol, status, search string) ([]service.Proxy, *pagination.PaginationResult, error) {
	q := r.client.Proxy.Query()
	if protocol != "" {
		q = q.Where(proxy.ProtocolEQ(protocol))
	}
	if status != "" {
		q = q.Where(proxy.StatusEQ(status))
	}
	if search != "" {
		q = q.Where(proxy.NameContainsFold(search))
	}

	total, err := q.Count(ctx)
	if err != nil {
		return nil, nil, err
	}

	proxiesQuery := q.
		Offset(params.Offset()).
		Limit(params.Limit())
	for _, order := range proxyListOrder(params) {
		proxiesQuery = proxiesQuery.Order(order)
	}

	proxies, err := proxiesQuery.All(ctx)
	if err != nil {
		return nil, nil, err
	}

	outProxies := make([]service.Proxy, 0, len(proxies))
	for i := range proxies {
		item := proxyEntityToService(proxies[i])
		if err := r.loadProxyExtraFields(ctx, item); err != nil {
			return nil, nil, err
		}
		outProxies = append(outProxies, *item)
	}

	return outProxies, paginationResultFromTotal(int64(total), params), nil
}

// ListWithFiltersAndAccountCount lists proxies with filters and includes account count per proxy
func (r *proxyRepository) ListWithFiltersAndAccountCount(ctx context.Context, params pagination.PaginationParams, protocol, status, search string) ([]service.ProxyWithAccountCount, *pagination.PaginationResult, error) {
	q := r.client.Proxy.Query()
	if protocol != "" {
		q = q.Where(proxy.ProtocolEQ(protocol))
	}
	if status != "" {
		q = q.Where(proxy.StatusEQ(status))
	}
	if search != "" {
		q = q.Where(proxy.NameContainsFold(search))
	}

	total, err := q.Count(ctx)
	if err != nil {
		return nil, nil, err
	}

	if strings.EqualFold(strings.TrimSpace(params.SortBy), "account_count") {
		return r.listWithAccountCountSort(ctx, q, params, total)
	}

	proxiesQuery := q.
		Offset(params.Offset()).
		Limit(params.Limit())
	for _, order := range proxyListOrder(params) {
		proxiesQuery = proxiesQuery.Order(order)
	}

	proxies, err := proxiesQuery.All(ctx)
	if err != nil {
		return nil, nil, err
	}

	return r.buildProxyWithAccountCountResult(ctx, proxies, params, int64(total))
}

func (r *proxyRepository) listWithAccountCountSort(ctx context.Context, q *dbent.ProxyQuery, params pagination.PaginationParams, total int) ([]service.ProxyWithAccountCount, *pagination.PaginationResult, error) {
	proxies, err := q.
		Order(dbent.Desc(proxy.FieldID)).
		All(ctx)
	if err != nil {
		return nil, nil, err
	}

	result, _, err := r.buildProxyWithAccountCountResult(ctx, proxies, params, int64(total))
	if err != nil {
		return nil, nil, err
	}

	sortOrder := params.NormalizedSortOrder(pagination.SortOrderDesc)
	sort.SliceStable(result, func(i, j int) bool {
		if result[i].AccountCount == result[j].AccountCount {
			return result[i].ID > result[j].ID
		}
		if sortOrder == pagination.SortOrderAsc {
			return result[i].AccountCount < result[j].AccountCount
		}
		return result[i].AccountCount > result[j].AccountCount
	})

	return paginateSlice(result, params), paginationResultFromTotal(int64(total), params), nil
}

func (r *proxyRepository) buildProxyWithAccountCountResult(ctx context.Context, proxies []*dbent.Proxy, params pagination.PaginationParams, total int64) ([]service.ProxyWithAccountCount, *pagination.PaginationResult, error) {
	counts, err := r.GetAccountCountsForProxies(ctx)
	if err != nil {
		return nil, nil, err
	}

	result := make([]service.ProxyWithAccountCount, 0, len(proxies))
	for i := range proxies {
		proxyOut := proxyEntityToService(proxies[i])
		if proxyOut == nil {
			continue
		}
		if err := r.loadProxyExtraFields(ctx, proxyOut); err != nil {
			return nil, nil, err
		}
		result = append(result, service.ProxyWithAccountCount{
			Proxy:        *proxyOut,
			AccountCount: counts[proxyOut.ID],
		})
	}

	return result, paginationResultFromTotal(total, params), nil
}

func proxyListOrder(params pagination.PaginationParams) []func(*entsql.Selector) {
	sortBy := strings.ToLower(strings.TrimSpace(params.SortBy))
	sortOrder := params.NormalizedSortOrder(pagination.SortOrderDesc)

	var field string
	switch sortBy {
	case "name":
		field = proxy.FieldName
	case "protocol":
		field = proxy.FieldProtocol
	case "status":
		field = proxy.FieldStatus
	case "created_at":
		field = proxy.FieldCreatedAt
	case "expiry":
		field = "expires_at"
	default:
		field = proxy.FieldID
	}

	if sortOrder == pagination.SortOrderAsc {
		return []func(*entsql.Selector){dbent.Asc(field), dbent.Asc(proxy.FieldID)}
	}
	return []func(*entsql.Selector){dbent.Desc(field), dbent.Desc(proxy.FieldID)}
}

func (r *proxyRepository) ListActive(ctx context.Context) ([]service.Proxy, error) {
	proxies, err := r.client.Proxy.Query().
		Where(proxy.StatusEQ(service.StatusActive)).
		All(ctx)
	if err != nil {
		return nil, err
	}
	outProxies := make([]service.Proxy, 0, len(proxies))
	for i := range proxies {
		item := proxyEntityToService(proxies[i])
		if err := r.loadProxyExtraFields(ctx, item); err != nil {
			return nil, err
		}
		outProxies = append(outProxies, *item)
	}
	return outProxies, nil
}

// ExistsByHostPortAuth checks if a proxy with the same host, port, username, and password exists
func (r *proxyRepository) ExistsByHostPortAuth(ctx context.Context, host string, port int, username, password string) (bool, error) {
	q := r.client.Proxy.Query().
		Where(proxy.HostEQ(host), proxy.PortEQ(port))

	if username == "" {
		q = q.Where(proxy.Or(proxy.UsernameIsNil(), proxy.UsernameEQ("")))
	} else {
		q = q.Where(proxy.UsernameEQ(username))
	}
	if password == "" {
		q = q.Where(proxy.Or(proxy.PasswordIsNil(), proxy.PasswordEQ("")))
	} else {
		q = q.Where(proxy.PasswordEQ(password))
	}

	count, err := q.Count(ctx)
	return count > 0, err
}

// CountAccountsByProxyID returns the number of accounts using a specific proxy
func (r *proxyRepository) CountAccountsByProxyID(ctx context.Context, proxyID int64) (int64, error) {
	var count int64
	if err := scanSingleRow(ctx, r.sql, "SELECT COUNT(*) FROM accounts WHERE proxy_id = $1 AND deleted_at IS NULL", []any{proxyID}, &count); err != nil {
		return 0, err
	}
	return count, nil
}

func (r *proxyRepository) ListAccountSummariesByProxyID(ctx context.Context, proxyID int64) ([]service.ProxyAccountSummary, error) {
	rows, err := r.sql.QueryContext(ctx, `
		SELECT id, name, platform, type, notes
		FROM accounts
		WHERE proxy_id = $1 AND deleted_at IS NULL
		ORDER BY id DESC
	`, proxyID)
	if err != nil {
		return nil, err
	}
	defer func() { _ = rows.Close() }()

	out := make([]service.ProxyAccountSummary, 0)
	for rows.Next() {
		var (
			id       int64
			name     string
			platform string
			accType  string
			notes    sql.NullString
		)
		if err := rows.Scan(&id, &name, &platform, &accType, &notes); err != nil {
			return nil, err
		}
		var notesPtr *string
		if notes.Valid {
			notesPtr = &notes.String
		}
		out = append(out, service.ProxyAccountSummary{
			ID:       id,
			Name:     name,
			Platform: platform,
			Type:     accType,
			Notes:    notesPtr,
		})
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}
	return out, nil
}

// GetAccountCountsForProxies returns a map of proxy ID to account count for all proxies
func (r *proxyRepository) GetAccountCountsForProxies(ctx context.Context) (counts map[int64]int64, err error) {
	rows, err := r.sql.QueryContext(ctx, "SELECT proxy_id, COUNT(*) AS count FROM accounts WHERE proxy_id IS NOT NULL AND deleted_at IS NULL GROUP BY proxy_id")
	if err != nil {
		return nil, err
	}
	defer func() {
		if closeErr := rows.Close(); closeErr != nil && err == nil {
			err = closeErr
			counts = nil
		}
	}()

	counts = make(map[int64]int64)
	for rows.Next() {
		var proxyID, count int64
		if err = rows.Scan(&proxyID, &count); err != nil {
			return nil, err
		}
		counts[proxyID] = count
	}
	if err = rows.Err(); err != nil {
		return nil, err
	}
	return counts, nil
}

// ListActiveWithAccountCount returns all active proxies with account count, sorted by creation time descending
func (r *proxyRepository) ListActiveWithAccountCount(ctx context.Context) ([]service.ProxyWithAccountCount, error) {
	proxies, err := r.client.Proxy.Query().
		Where(proxy.StatusEQ(service.StatusActive)).
		Order(dbent.Desc(proxy.FieldCreatedAt)).
		All(ctx)
	if err != nil {
		return nil, err
	}

	// Get account counts
	counts, err := r.GetAccountCountsForProxies(ctx)
	if err != nil {
		return nil, err
	}

	// Build result with account counts
	result := make([]service.ProxyWithAccountCount, 0, len(proxies))
	for i := range proxies {
		proxyOut := proxyEntityToService(proxies[i])
		if proxyOut == nil {
			continue
		}
		if err := r.loadProxyExtraFields(ctx, proxyOut); err != nil {
			return nil, err
		}
		result = append(result, service.ProxyWithAccountCount{
			Proxy:        *proxyOut,
			AccountCount: counts[proxyOut.ID],
		})
	}

	return result, nil
}

func proxyEntityToService(m *dbent.Proxy) *service.Proxy {
	if m == nil {
		return nil
	}
	out := &service.Proxy{
		ID:        m.ID,
		Name:      m.Name,
		Protocol:  m.Protocol,
		Host:      m.Host,
		Port:      m.Port,
		Status:    m.Status,
		CreatedAt: m.CreatedAt,
		UpdatedAt: m.UpdatedAt,
	}
	if m.Username != nil {
		out.Username = *m.Username
	}
	if m.Password != nil {
		out.Password = *m.Password
	}
	out.FallbackMode = service.FallbackModeNone
	out.ExpiryWarnDays = 7
	return out
}

func applyProxyEntityToService(dst *service.Proxy, src *dbent.Proxy) {
	if dst == nil || src == nil {
		return
	}
	dst.ID = src.ID
	dst.CreatedAt = src.CreatedAt
	dst.UpdatedAt = src.UpdatedAt
}

func (r *proxyRepository) saveProxyExtraFields(ctx context.Context, p *service.Proxy) error {
	if r.sql == nil || p == nil || p.ID == 0 {
		return nil
	}
	mode := normalizeProxyFallbackMode(p.FallbackMode)
	warnDays := p.ExpiryWarnDays
	if warnDays <= 0 {
		warnDays = 7
	}
	_, err := r.sql.ExecContext(ctx, `
		UPDATE proxies
		SET expires_at=$1, fallback_mode=$2, backup_proxy_id=$3, expiry_warn_days=$4, updated_at=NOW()
		WHERE id=$5 AND deleted_at IS NULL
	`, p.ExpiresAt, mode, p.BackupProxyID, warnDays, p.ID)
	if err != nil {
		return err
	}
	p.FallbackMode = mode
	p.ExpiryWarnDays = warnDays
	return nil
}

func (r *proxyRepository) loadProxyExtraFields(ctx context.Context, p *service.Proxy) error {
	if r.sql == nil || p == nil || p.ID == 0 {
		return nil
	}
	var (
		expiresAt     sql.NullTime
		fallbackMode  sql.NullString
		backupProxyID sql.NullInt64
		warnDays      sql.NullInt64
	)
	if err := scanSingleRow(ctx, r.sql, `
		SELECT expires_at, fallback_mode, backup_proxy_id, expiry_warn_days
		FROM proxies
		WHERE id=$1 AND deleted_at IS NULL
	`, []any{p.ID}, &expiresAt, &fallbackMode, &backupProxyID, &warnDays); err != nil {
		return err
	}
	if expiresAt.Valid {
		t := expiresAt.Time
		p.ExpiresAt = &t
	} else {
		p.ExpiresAt = nil
	}
	if fallbackMode.Valid {
		p.FallbackMode = normalizeProxyFallbackMode(fallbackMode.String)
	} else {
		p.FallbackMode = service.FallbackModeNone
	}
	if backupProxyID.Valid {
		id := backupProxyID.Int64
		p.BackupProxyID = &id
	} else {
		p.BackupProxyID = nil
	}
	if warnDays.Valid {
		p.ExpiryWarnDays = int(warnDays.Int64)
	} else {
		p.ExpiryWarnDays = 7
	}
	return nil
}

func normalizeProxyFallbackMode(mode string) string {
	switch strings.TrimSpace(mode) {
	case service.FallbackModeProxy:
		return service.FallbackModeProxy
	case service.FallbackModeDirect:
		return service.FallbackModeDirect
	default:
		return service.FallbackModeNone
	}
}

// ListAllForFallback returns every non-deleted proxy, including inactive and
// expired proxies, so fallback chain resolution has a complete snapshot.
func (r *proxyRepository) ListAllForFallback(ctx context.Context) ([]service.Proxy, error) {
	if r.sql == nil {
		return []service.Proxy{}, nil
	}
	rows, err := r.sql.QueryContext(ctx, `
		SELECT id, name, protocol, host, port, username, password, status, created_at, updated_at,
		       expires_at, fallback_mode, backup_proxy_id, expiry_warn_days
		FROM proxies
		WHERE deleted_at IS NULL
	`)
	if err != nil {
		return nil, err
	}
	defer func() { _ = rows.Close() }()

	out := make([]service.Proxy, 0)
	for rows.Next() {
		p, err := scanProxyFallbackRow(rows)
		if err != nil {
			return nil, err
		}
		out = append(out, *p)
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}
	return out, nil
}

func scanProxyFallbackRow(rows *sql.Rows) (*service.Proxy, error) {
	var (
		p             service.Proxy
		username      sql.NullString
		password      sql.NullString
		expiresAt     sql.NullTime
		fallbackMode  sql.NullString
		backupProxyID sql.NullInt64
		warnDays      sql.NullInt64
	)
	if err := rows.Scan(
		&p.ID, &p.Name, &p.Protocol, &p.Host, &p.Port, &username, &password, &p.Status, &p.CreatedAt, &p.UpdatedAt,
		&expiresAt, &fallbackMode, &backupProxyID, &warnDays,
	); err != nil {
		return nil, err
	}
	if username.Valid {
		p.Username = username.String
	}
	if password.Valid {
		p.Password = password.String
	}
	if expiresAt.Valid {
		t := expiresAt.Time
		p.ExpiresAt = &t
	}
	if fallbackMode.Valid {
		p.FallbackMode = normalizeProxyFallbackMode(fallbackMode.String)
	} else {
		p.FallbackMode = service.FallbackModeNone
	}
	if backupProxyID.Valid {
		id := backupProxyID.Int64
		p.BackupProxyID = &id
	}
	if warnDays.Valid {
		p.ExpiryWarnDays = int(warnDays.Int64)
	} else {
		p.ExpiryWarnDays = 7
	}
	return &p, nil
}

func (r *proxyRepository) SweepExpiredProxies(ctx context.Context, now time.Time) (int64, error) {
	all, err := r.ListAllForFallback(ctx)
	if err != nil {
		return 0, err
	}
	byID := make(map[int64]service.Proxy, len(all))
	for _, p := range all {
		byID[p.ID] = p
	}

	var totalChanged int64
	accountsTouched := false
	for _, p := range all {
		if p.Status != service.StatusActive || !p.IsExpired(now) {
			continue
		}
		target, change := service.ResolveProxyFallbackTarget(p, byID, now)
		if !change && p.FallbackMode == service.FallbackModeProxy {
			logger.LegacyPrintf("repository.proxy", "[ProxyExpiry] proxy %d expired but fallback chain unresolved; accounts kept", p.ID)
		}
		changed, sweepErr := r.sweepOneExpiredProxy(ctx, p.ID, target, change)
		if sweepErr != nil {
			return totalChanged, sweepErr
		}
		if changed > 0 {
			totalChanged += changed
			accountsTouched = true
		}
	}
	if accountsTouched {
		if err := enqueueSchedulerOutbox(ctx, r.sql, service.SchedulerOutboxEventFullRebuild, nil, nil, nil); err != nil {
			logger.LegacyPrintf("repository.proxy", "[SchedulerOutbox] enqueue proxy expiry rebuild failed: err=%v", err)
		}
	}
	return totalChanged, nil
}

func (r *proxyRepository) sweepOneExpiredProxy(ctx context.Context, proxyID int64, target *int64, change bool) (int64, error) {
	if r.sql == nil {
		return 0, nil
	}
	if _, err := r.sql.ExecContext(ctx,
		`UPDATE proxies SET status=$1, updated_at=NOW() WHERE id=$2 AND deleted_at IS NULL`,
		service.StatusExpired, proxyID); err != nil {
		return 0, err
	}
	if !change {
		return 0, nil
	}
	var (
		res sql.Result
		err error
	)
	if target == nil {
		res, err = r.sql.ExecContext(ctx, `
			UPDATE accounts SET proxy_id=NULL, proxy_fallback_origin_id=$1, updated_at=NOW()
			WHERE proxy_id=$1 AND proxy_fallback_origin_id IS NULL AND deleted_at IS NULL`, proxyID)
	} else {
		res, err = r.sql.ExecContext(ctx, `
			UPDATE accounts SET proxy_id=$2, proxy_fallback_origin_id=$1, updated_at=NOW()
			WHERE proxy_id=$1 AND proxy_fallback_origin_id IS NULL AND deleted_at IS NULL`, proxyID, *target)
	}
	if err != nil {
		return 0, err
	}
	n, _ := res.RowsAffected()
	return n, nil
}

func (r *proxyRepository) CountExpired(ctx context.Context) (int64, error) {
	if r.sql == nil {
		return 0, nil
	}
	var count int64
	err := scanSingleRow(ctx, r.sql, `SELECT COUNT(*) FROM proxies WHERE status=$1 AND deleted_at IS NULL`, []any{service.StatusExpired}, &count)
	return count, err
}

func (r *proxyRepository) CountExpiringSoon(ctx context.Context, now time.Time) (int64, error) {
	if r.sql == nil {
		return 0, nil
	}
	var count int64
	err := scanSingleRow(ctx, r.sql, `
		SELECT COUNT(*) FROM proxies
		WHERE deleted_at IS NULL AND status=$1 AND expires_at IS NOT NULL
		  AND expires_at > $2 AND expires_at <= $2 + (expiry_warn_days || ' days')::interval
	`, []any{service.StatusActive, now}, &count)
	return count, err
}
