package model

type Target struct {
	Host string
	Port string
}

type UpstreamConfig struct {
	Host string
	Port int
}

type ProxyRequest struct {
	Method  string
	URL     string
	Target  Target
	Headers map[string][]string
	Body    []byte
	IsTLS   bool
}

type ProxyResponse struct {
	StatusCode int
	Headers    map[string][]string
	Body       []byte
}
