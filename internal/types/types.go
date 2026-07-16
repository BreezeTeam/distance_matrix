package types

type APIResponse struct {
	Code int    `json:"code"`
	Msg  string `json:"msg"`
}

type MatrixRequest struct {
	Points     [][]float32 `json:"points"`
	Coordinate string      `json:"coordinate,optional"`
	Strategy   int         `json:"strategy,optional"`
	Method     int         `json:"method,optional"`
	TimeSlot   string      `json:"timeslot,optional"`
	Strict     bool        `json:"strict,optional"`
	GeoWideM   int         `json:"geo_wide_m,optional"`
	Provider   string      `json:"provider,optional"`
	SpeedMPS   int         `json:"speed_mps,optional"`
}

type MatrixData struct {
	Distances [][]float32 `json:"distances"`
	Durations [][]float32 `json:"durations"`
}

type MatrixResponse struct {
	Code int         `json:"code"`
	Msg  string      `json:"msg"`
	Data *MatrixData `json:"data,omitempty"`
}

type RouteRequest struct {
	Points     [][]float32 `json:"points"`
	Coordinate string      `json:"coordinate,optional"`
	Strategy   int         `json:"strategy,optional"`
	Method     int         `json:"method,optional"`
	Provider   string      `json:"provider,optional"`
	SpeedMPS   int         `json:"speed_mps,optional"`
}

type RouteStep struct {
	Origin      []float32 `json:"origin"`
	Destination []float32 `json:"destination"`
	Distance    float32   `json:"distance"`
	Duration    float32   `json:"duration"`
}

type RouteData struct {
	Distance float32     `json:"distance"`
	Duration float32     `json:"duration"`
	Steps    []RouteStep `json:"steps"`
}

type RouteResponse struct {
	Code int        `json:"code"`
	Msg  string     `json:"msg"`
	Data *RouteData `json:"data,omitempty"`
}

type ProvidersResponse struct {
	Code int      `json:"code"`
	Msg  string   `json:"msg"`
	Data []string `json:"data,omitempty"`
}
