package storageos

// APi defaults
const (
	DefaultAPIHost   = "localhost"
	DefaultAPIPort   = "8000"
	DefaultAPIScheme = "http"
	APIVersion       = "v1"
)

// API endpoints
const (
	ControllerEndpoint = "controller"
	VolumeEndpoint     = "volumes"
)

// Errors
const (
	ErrNotFound         = "Not found"
	ErrHTTPOther        = "API returned error"
	ErrReadResponseBody = "Failed to read response body"
	ErrJSONUnMarshall   = "Failed to parse response body"
)
