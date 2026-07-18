package persist

import (
	"context"
	"fmt"
	"log"
	"os"
	"strings"
	"time"

	"distance-matrix/internal/cache"

	"gorm.io/driver/mysql"
	"gorm.io/gorm"
	"gorm.io/gorm/clause"
	"gorm.io/gorm/logger"
)

// OpenOptions for MySQL archive. AutoMigrate=false by default (no CREATE rights required).
type OpenOptions struct {
	DSN          string
	Database     string
	MaxOpenConns int
	MaxIdleConns int
	AutoMigrate  bool
}

// Store is the GORM-backed edge archive.
type Store struct {
	db *gorm.DB
}

// Open connects with GORM. AutoMigrate uses GORM Migrator (no hand-written DDL).
func Open(opts OpenOptions) (*Store, error) {
	dsn := strings.TrimSpace(opts.DSN)
	if dsn == "" {
		return nil, fmt.Errorf("persist: empty DSN")
	}
	maxOpen := opts.MaxOpenConns
	if maxOpen <= 0 {
		maxOpen = 10
	}
	maxIdle := opts.MaxIdleConns
	if maxIdle <= 0 {
		maxIdle = 5
	}

	dbName := strings.TrimSpace(opts.Database)
	if dbName == "" {
		dbName = databaseFromDSN(dsn)
	}
	if opts.AutoMigrate && dbName != "" {
		if err := ensureDatabase(dsn, dbName); err != nil {
			return nil, fmt.Errorf("persist: create database: %w", err)
		}
	}
	if dbName != "" {
		dsn = withDatabase(dsn, dbName)
	}

	db, err := gorm.Open(mysql.Open(dsn), &gorm.Config{
		Logger: logger.New(log.New(os.Stdout, "\r\n", log.LstdFlags), logger.Config{
			SlowThreshold:             200 * time.Millisecond,
			LogLevel:                  logger.Warn,
			IgnoreRecordNotFoundError: true, // L2 miss is normal
			Colorful:                  false,
		}),
	})
	if err != nil {
		return nil, fmt.Errorf("persist: gorm open: %w", err)
	}
	sqlDB, err := db.DB()
	if err != nil {
		return nil, err
	}
	sqlDB.SetMaxOpenConns(maxOpen)
	sqlDB.SetMaxIdleConns(maxIdle)
	sqlDB.SetConnMaxLifetime(time.Hour)

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	if err := sqlDB.PingContext(ctx); err != nil {
		_ = sqlDB.Close()
		return nil, fmt.Errorf("persist: ping: %w", err)
	}

	if opts.AutoMigrate {
		if err := db.WithContext(ctx).AutoMigrate(&DistanceMatrixEdge{}); err != nil {
			_ = sqlDB.Close()
			return nil, fmt.Errorf("persist: AutoMigrate: %w", err)
		}
	} else if !db.Migrator().HasTable(&DistanceMatrixEdge{}) {
		_ = sqlDB.Close()
		var m DistanceMatrixEdge
		return nil, fmt.Errorf("persist: table %s missing — run with Persistence.AutoMigrate=true (privileged user) or: go run ./scripts/genddl -dsn '...'", m.TableName())
	}

	return &Store{db: db}, nil
}

// Migrate runs GORM AutoMigrate for DistanceMatrixEdge.
func (s *Store) Migrate(ctx context.Context) error {
	if s == nil || s.db == nil {
		return fmt.Errorf("persist: nil store")
	}
	return s.db.WithContext(ctx).AutoMigrate(&DistanceMatrixEdge{})
}

func ensureDatabase(dsn, dbName string) error {
	rootDSN := withDatabase(dsn, "")
	db, err := gorm.Open(mysql.Open(rootDSN), &gorm.Config{
		Logger: logger.Default.LogMode(logger.Silent),
	})
	if err != nil {
		return err
	}
	sqlDB, err := db.DB()
	if err != nil {
		return err
	}
	defer sqlDB.Close()
	return db.Exec("CREATE DATABASE IF NOT EXISTS `" + strings.ReplaceAll(dbName, "`", "``") + "`").Error
}

func databaseFromDSN(dsn string) string {
	i := strings.Index(dsn, ")/")
	if i < 0 {
		return ""
	}
	rest := dsn[i+2:]
	if rest == "" || strings.HasPrefix(rest, "?") {
		return ""
	}
	name := rest
	if j := strings.IndexAny(rest, "?/"); j >= 0 {
		name = rest[:j]
	}
	return name
}

func withDatabase(dsn, dbName string) string {
	i := strings.Index(dsn, ")/")
	if i < 0 {
		return dsn
	}
	prefix := dsn[:i+2]
	rest := dsn[i+2:]
	params := ""
	if j := strings.Index(rest, "?"); j >= 0 {
		params = rest[j:]
	}
	return prefix + dbName + params
}

func (s *Store) Close() error {
	if s == nil || s.db == nil {
		return nil
	}
	sqlDB, err := s.db.DB()
	if err != nil {
		return err
	}
	return sqlDB.Close()
}

// Upsert inserts or updates one edge. Skips zero-distance.
func (s *Store) Upsert(ctx context.Context, opts cache.LookupOpts, e cache.Edge) error {
	if s == nil || s.db == nil {
		return nil
	}
	if e.DistanceM == 0 {
		return nil
	}
	tenant := opts.Tenant
	if tenant == "" {
		tenant = "default"
	}
	wmt := e.WMT
	if wmt == "" {
		wmt = cache.TimeSlotWMH(time.Now())
	}
	nowUnix := uint64(time.Now().Unix())
	row := DistanceMatrixEdge{
		Tenant:      tenant,
		Method:      int32(opts.Method),
		Strategy:    int32(opts.Strategy),
		StartGeo:    cache.EncodeGeoHash(e.Origin[0], e.Origin[1]),
		EndGeo:      cache.EncodeGeoHash(e.Destination[0], e.Destination[1]),
		WMT:         wmt,
		T:           cache.HourBucket(wmt),
		Origin:      coordJSON(e.Origin),
		Destination: coordJSON(e.Destination),
		Distance:    float64(e.DistanceM),
		Duration:    float64(e.DurationS),
		Polyline:    e.Polyline,
		Provider:    e.Provider,
		CreatedAt:   nowUnix,
		UpdatedAt:   nowUnix,
	}
	return s.db.WithContext(ctx).Clauses(clause.OnConflict{
		Columns: []clause.Column{
			{Name: "tenant"}, {Name: "method"}, {Name: "strategy"},
			{Name: "start_geo"}, {Name: "end_geo"}, {Name: "w_m_t"},
		},
		DoUpdates: clause.Assignments(map[string]any{
			"origin":      row.Origin,
			"destination": row.Destination,
			"distance":    row.Distance,
			"duration":    row.Duration,
			"polyline":    row.Polyline,
			"provider":    row.Provider,
			"t":           row.T,
			"updated_at":  row.UpdatedAt,
			"version":     gorm.Expr("version + 1"),
		}),
	}).Create(&row).Error
}

// Get loads a cold edge. Strict → exact w_m_t; else exact → ±1h slots → same hour bucket t.
func (s *Store) Get(ctx context.Context, opts cache.LookupOpts, origin, destination [2]float32) (cache.Edge, bool, error) {
	if s == nil || s.db == nil {
		return cache.Edge{}, false, nil
	}
	tenant := opts.Tenant
	if tenant == "" {
		tenant = "default"
	}
	slot := opts.TimeSlot
	if slot == "" {
		slot = cache.TimeSlotWMH(time.Now())
	}
	start := cache.EncodeGeoHash(origin[0], origin[1])
	end := cache.EncodeGeoHash(destination[0], destination[1])

	slots := []string{slot}
	if !opts.Strict {
		slots = cache.AdjacentSlots(slot)
	}
	if edge, ok, err := s.getBySlots(ctx, tenant, opts.Method, opts.Strategy, start, end, slots); err != nil || ok {
		return edge, ok, err
	}
	if opts.Strict {
		return cache.Edge{}, false, nil
	}
	return s.getByHour(ctx, tenant, opts.Method, opts.Strategy, start, end, cache.HourBucket(slot), origin, destination)
}

func (s *Store) getBySlots(ctx context.Context, tenant string, method, strategy int, start, end string, slots []string) (cache.Edge, bool, error) {
	var rows []DistanceMatrixEdge
	err := s.db.WithContext(ctx).
		Where("tenant = ? AND method = ? AND strategy = ? AND start_geo = ? AND end_geo = ? AND w_m_t IN ?",
			tenant, method, strategy, start, end, slots).
		Find(&rows).Error
	if err != nil {
		return cache.Edge{}, false, err
	}
	bySlot := make(map[string]DistanceMatrixEdge, len(rows))
	for _, r := range rows {
		bySlot[r.WMT] = r
	}
	for _, sl := range slots {
		if r, ok := bySlot[sl]; ok {
			return rowToEdge(r), true, nil
		}
	}
	return cache.Edge{}, false, nil
}

func (s *Store) getByHour(ctx context.Context, tenant string, method, strategy int, start, end, t string, origin, destination [2]float32) (cache.Edge, bool, error) {
	var rows []DistanceMatrixEdge
	err := s.db.WithContext(ctx).
		Where("tenant = ? AND method = ? AND strategy = ? AND start_geo = ? AND end_geo = ? AND t = ?",
			tenant, method, strategy, start, end, t).
		Order("updated_at DESC, gmt_modified DESC").
		Limit(1).
		Find(&rows).Error
	if err != nil {
		return cache.Edge{}, false, err
	}
	if len(rows) == 0 {
		return cache.Edge{}, false, nil
	}
	edge := rowToEdge(rows[0])
	if edge.Origin == ([2]float32{}) {
		edge.Origin = origin
	}
	if edge.Destination == ([2]float32{}) {
		edge.Destination = destination
	}
	return edge, true, nil
}

func rowToEdge(r DistanceMatrixEdge) cache.Edge {
	o, _ := parseCoord(r.Origin)
	d, _ := parseCoord(r.Destination)
	return cache.Edge{
		Origin: o, Destination: d,
		DistanceM: float32(r.Distance), DurationS: float32(r.Duration),
		Polyline: r.Polyline, WMT: r.WMT, Provider: r.Provider,
	}
}
