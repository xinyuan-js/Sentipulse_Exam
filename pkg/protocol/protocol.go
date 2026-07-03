package protocol

type PluginRequest struct {
	Data map[string]any `json:"data"`
}

type PluginResponse struct {
	Result map[string]any `json:"result,omitempty"`
	Error  string         `json:"error,omitempty"`
}
