package provider

// resourceModels and dataSourceModels map each Terraform type name to the
// struct the provider decodes state into. Adding a resource means adding it
// here, which is what keeps TestSchemaMatchesModel honest.

var resourceModels = map[string]any{
	"partiteai_agent":                      AgentResourceModel{},
	"partiteai_api_key":                    ApiKeyResourceModel{},
	"partiteai_embedding_profile":          EmbeddingProfileResourceModel{},
	"partiteai_installed_particle":         InstalledParticleResourceModel{},
	"partiteai_mcp_connection":             McpConnectionResourceModel{},
	"partiteai_memory_interaction_profile": MemoryInteractionProfileResourceModel{},
	"partiteai_memory_store":               MemoryStoreResourceModel{},
	"partiteai_mesh":                       MeshResourceModel{},
	"partiteai_mesh_input":                 MeshInputResourceModel{},
	"partiteai_mesh_link":                  MeshLinkResourceModel{},
	"partiteai_model_profile":              ModelProfileResourceModel{},
	"partiteai_model_provider":             ModelProviderResourceModel{},
	"partiteai_model_set":                  ModelSetResourceModel{},
	"partiteai_particle_artifact":          ParticleArtifactResourceModel{},
	"partiteai_task_definition":            TaskDefinitionResourceModel{},
	"partiteai_webhook_destination":        WebhookDestinationResourceModel{},
	"partiteai_workspace":                  WorkspaceResourceModel{},
	"partiteai_user_metric":                UserMetricResourceModel{},
	"partiteai_rubric":                     RubricResourceModel{},
	"partiteai_eval_mesh_monitor":          EvalMeshMonitorResourceModel{},
}

var dataSourceModels = map[string]any{
	"partiteai_agent":                      AgentDataSourceModel{},
	"partiteai_api_key":                    ApiKeyDataSourceModel{},
	"partiteai_embedding_profile":          EmbeddingProfileDataSourceModel{},
	"partiteai_installed_particle":         InstalledParticleDataSourceModel{},
	"partiteai_mcp_connection":             McpConnectionDataSourceModel{},
	"partiteai_memory_interaction_profile": MemoryInteractionProfileDataSourceModel{},
	"partiteai_memory_store":               MemoryStoreDataSourceModel{},
	"partiteai_mesh":                       MeshDataSourceModel{},
	"partiteai_mesh_input":                 MeshInputDataSourceModel{},
	"partiteai_mesh_link":                  MeshLinkDataSourceModel{},
	"partiteai_model_profile":              ModelProfileDataSourceModel{},
	"partiteai_model_provider":             ModelProviderDataSourceModel{},
	"partiteai_model_set":                  ModelSetDataSourceModel{},
	"partiteai_particle_artifact":          ParticleArtifactDataSourceModel{},
	"partiteai_task_definition":            TaskDefinitionDataSourceModel{},
	"partiteai_webhook_destination":        WebhookDestinationDataSourceModel{},
	"partiteai_workspace":                  WorkspaceDataSourceModel{},
	"partiteai_user_metric":                UserMetricDataSourceModel{},
	"partiteai_rubric":                     RubricDataSourceModel{},
	"partiteai_eval_mesh_monitor":          EvalMeshMonitorDataSourceModel{},
	"partiteai_system_metric":              SystemMetricDataSourceModel{},
}
