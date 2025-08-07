package config

// Extention to the segment config definition. Adding branch conditions.
type BranchOptions struct {
	If   []SegmentRepr `yaml:"if,omitempty,flow"`
	Then []SegmentRepr `yaml:"then,omitempty,flow"`
	Else []SegmentRepr `yaml:"else,omitempty,flow"`
}
