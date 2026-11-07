package models

type Move struct {
	Key           string `json:"key"`
	Command       string `json:"move_name"` // keep legacy JSON key
	Startup       string `json:"startup"`
	HitLevel      string `json:"hit_level"`
	FramesOnBlock string `json:"frames_on_block"`
	FramesOnHit   string `json:"frames_on_hit"`
	Notes         string `json:"notes"`
}
