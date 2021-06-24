package tva

type metadataType string

type MetadataRequest struct {
	Metadata Metadata `json:"metadata"`
}

type Metadata struct {
	Labels      map[string]*string `json:"labels,omitempty"`
	Annotations map[string]*string `json:"annotations,omitempty"`
}
