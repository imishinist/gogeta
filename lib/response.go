package gogeta

type Response struct {
	Name    string
	Latency int64 // ms

	QueryType QueryType
	Query     string `json:"Query,omitempty"`
	ExecQuery string `json:"ExecQuery,omitempty"`

	LastInsertId int64
	RowsAffected int64

	ResultCount int
	Results     []map[string]interface{} `json:"Results,omitempty"`

	Error error
}
