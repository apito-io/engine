package models

type ProjectApiTracking map[string]ApiTracking

type ApiTracking struct {
	Increment uint32
	Bandwidth float64
}
