package vcap

// Application holds an apps CF config
type Application struct {
	CFAPI  string `json:"cf_api"`
	Limits struct {
		Fds  int `json:"fds"`
		Mem  int `json:"mem"`
		Disk int `json:"disk"`
	} `json:"limits"`
	ApplicationName    string   `json:"application_name"`
	ApplicationUris    []string `json:"application_uris"`
	Name               string   `json:"name"`
	SpaceName          string   `json:"space_name"`
	SpaceID            string   `json:"space_id"`
	OrganizationID     string   `json:"organization_id"`
	OrganizationName   string   `json:"organization_name"`
	Uris               []string `json:"uris"`
	ProcessID          string   `json:"process_id"`
	ProcessType        string   `json:"process_type"`
	ApplicationID      string   `json:"application_id"`
	Version            string   `json:"version"`
	ApplicationVersion string   `json:"application_version"`
}
