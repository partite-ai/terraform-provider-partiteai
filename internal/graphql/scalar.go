package graphql

type Tag struct {
	Name  string `json:"name" tfsdk:"name"`
	Value string `json:"value" tfsdk:"value"`
}
