package params

type SqueezeParam struct {
	BBL int     `json:"bbl"` // BB Length
	BBF float64 `json:"bbf"` // BB MultiFactor
	KCL int     `json:"kcl"` // KC Length
	KCF float64 `json:"kcf"` // KC MultiFactor
}

type SuperTrendParam struct {
	Factor float64
	Period int
}
