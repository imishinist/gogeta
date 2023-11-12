package gogeta

//go:generate enumer -type=QueryType -json
type QueryType int

const (
	Query QueryType = iota
	Execute
)

type Request struct {
	Name string

	QueryType QueryType
	Query     string `json:"Query,omitempty"`
	ExecQuery string `json:"ExecQuery,omitempty"`

	Prepare bool
}
