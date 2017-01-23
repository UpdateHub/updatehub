package metadata

type Metadata struct {
	ProductUID string `json:"product-uid"`
	Version    string `json:"version"`
	Objects    [][]Object
}
