// LOGZ.IO GRAFANA CHANGE :: DEV-17927 use LogzIoHeaders obj to pass on headers
package models

const (
	AuthTokenHeader   = "X-AUTH-TOKEN"
	ApiTokenHeader    = "X-API-TOKEN"
	UserContextHeader = "USER-CONTEXT"
)

type LogzIoHeaders struct {
	AuthToken   string
	ApiToken    string
	UserContext string
}

func (h *LogzIoHeaders) GetAuthHeader() (string, string) {
	if h.ApiToken != "" {
		return ApiTokenHeader, h.ApiToken
	} else if h.AuthToken != "" {
		return AuthTokenHeader, h.AuthToken
	} else if h.UserContext != "" {
		return UserContextHeader, h.UserContext
	}

	return "", ""
}

// LOGZ.IO GRAFANA CHANGE :: end
