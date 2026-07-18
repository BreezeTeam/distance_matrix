package persist

import "time"

// DistanceMatrixEdge is the GORM model for distance_matrix_edge.
// Schema changes: edit this model, then either:
//   - Persistence.AutoMigrate=true (privileged DB user), or
//   - go run ./scripts/genddl -dsn 'user:pass@tcp(host:3306)/distance_matrix?parseTime=true&charset=utf8mb4'
type DistanceMatrixEdge struct {
	ID uint64 `gorm:"column:id;type:bigint unsigned;primaryKey;autoIncrement;comment:auto-increment primary key"`

	Tenant   string `gorm:"column:tenant;type:varchar(64);not null;default:default;uniqueIndex:uk_distance_matrix_edge_ctx;index:idx_distance_matrix_edge_hour,priority:1;comment:tenant id"`
	Method   int32  `gorm:"column:method;type:int;not null;default:0;uniqueIndex:uk_distance_matrix_edge_ctx;index:idx_distance_matrix_edge_hour,priority:2;comment:travel method context"`
	Strategy int32  `gorm:"column:strategy;type:int;not null;default:0;uniqueIndex:uk_distance_matrix_edge_ctx;index:idx_distance_matrix_edge_hour,priority:3;comment:routing strategy context"`
	StartGeo string `gorm:"column:start_geo;type:varchar(16);not null;default:'';uniqueIndex:uk_distance_matrix_edge_ctx;index:idx_distance_matrix_edge_hour,priority:4;comment:origin geohash"`
	EndGeo   string `gorm:"column:end_geo;type:varchar(16);not null;default:'';uniqueIndex:uk_distance_matrix_edge_ctx;index:idx_distance_matrix_edge_hour,priority:5;comment:destination geohash"`
	WMT      string `gorm:"column:w_m_t;type:varchar(8);not null;default:'';uniqueIndex:uk_distance_matrix_edge_ctx;comment:time slot weekday+month+hour"`
	T        string `gorm:"column:t;type:varchar(2);not null;default:'';index:idx_distance_matrix_edge_hour,priority:6;comment:hour bucket (last two digits of WMH)"`

	Origin      string  `gorm:"column:origin;type:varchar(64);not null;default:'';comment:origin coords JSON [lon,lat]"`
	Destination string  `gorm:"column:destination;type:varchar(64);not null;default:'';comment:destination coords JSON [lon,lat]"`
	Distance    float64 `gorm:"column:distance;type:double;not null;default:0;comment:distance in meters"`
	Duration    float64 `gorm:"column:duration;type:double;not null;default:0;comment:duration in seconds"`
	Polyline    string  `gorm:"column:polyline;type:longtext;comment:route polyline"`
	Provider    string  `gorm:"column:provider;type:varchar(64);not null;default:'';comment:source provider"`

	CreatedAt uint64 `gorm:"column:created_at;type:bigint unsigned;not null;default:0;comment:created at (unix seconds)"`
	UpdatedAt uint64 `gorm:"column:updated_at;type:bigint unsigned;not null;default:0;index:idx_distance_matrix_edge_updated_at;comment:updated at (unix seconds)"`

	// Mandatory audit columns.
	GmtCreate   time.Time `gorm:"column:gmt_create;type:datetime;not null;autoCreateTime;comment:row create time"`
	GmtModified time.Time `gorm:"column:gmt_modified;type:datetime;not null;autoUpdateTime;comment:row modify time"`
	Creator     string    `gorm:"column:creator;type:varchar(50);not null;default:'';comment:creator"`
	Modifier    string    `gorm:"column:modifier;type:varchar(50);not null;default:'';comment:modifier"`
	Version     uint32    `gorm:"column:version;type:int unsigned;not null;default:0;comment:optimistic version"`
}

func (DistanceMatrixEdge) TableName() string { return "distance_matrix_edge" }
