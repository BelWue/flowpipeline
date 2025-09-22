package lumberjack

type ECSRelated struct {
	Hash  []string `json:"hash,omitempty"`
	Hosts []string `json:"hosts,omitempty"`
	IP    []string `json:"ip,omitempty"`
	User  []string `json:"user,omitempty"`
}
