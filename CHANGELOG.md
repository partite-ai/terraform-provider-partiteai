## 0.1.0 (Unreleased)

BREAKING CHANGES:

* resource/partiteai_mcp_connection: `stdio_transport` is removed. Mesh no
  longer runs MCP servers as child processes, so there is nothing behind it.
  Replace a stdio connection with `tunnel_transport`, and run the server
  yourself with `partite-mcp-relay`, which launches it over stdio the same way
  and reaches mesh over an outbound tunnel.
* data-source/partiteai_mcp_connection: `stdio_transport` is removed, for the
  same reason.
* resource/partiteai_user_metric is now `partiteai_judge_metric`, and
  data-source/partiteai_user_metric is now `partiteai_judge_metric`. Every
  judged metric is one kind now, so the old split between metrics of your own
  and ones that came with the product is gone. A judge metric requires
  `category` and `scope`.
* data-source/partiteai_system_metric is removed. The judged metrics it read are
  now judge metrics every organization starts with, editable like any other:
  find one by name with the `partiteai_judge_metric` data source. The computed
  ones are read with the new `partiteai_computed_metric` data source.
* resource/partiteai_rubric: in `metrics`, `user_metric_id` is now
  `judge_metric_id` and `system_metric_key` is now `computed_metric_key`. A
  setting that does not apply to the kind of metric is refused at plan time:
  `trial_count`, `complexity` and `input_values` are for judge metrics, and the
  score bounds for computed ones. `input_values` can't be an empty object; leave
  it out instead.
* data-source/partiteai_rubric: the same renames in `metrics`.
* data-source/partiteai_model_provider: the server no longer returns provider
  secrets, so `api_key`, `api_token` and `service_account_key` are removed.
  Each is replaced by a boolean saying whether it is set: `api_key_set`,
  `api_token_set` and `service_account_key_set`.
* resource/partiteai_model_provider: secrets are write-only and never stored in
  state. `api_key` is now `api_key_wo`, `api_token` is `api_token_wo` and
  `service_account_key` is `service_account_key_wo`. Terraform can't see a
  write-only value change, so to send a new secret change the new
  `credentials_version` attribute (default 1) along with it. An update that
  changes neither it nor the provider's other settings leaves the stored
  secret alone. This also makes the resource work against servers that no
  longer return provider secrets.

FEATURES:

* **New Resource:** `partiteai_skill`. A skill is a folder of text files, given
  as `files` by path, `SKILL.md` among them. Its own `name` is unique; the name
  `SKILL.md` declares is what agents see.
* **New Data Source:** `partiteai_skill`, found by `id` or `name`.
* resource/partiteai_agent: `skills` gives an agent skills to load. No two of an
  agent's skills can declare the same name.
* data-source/partiteai_agent: `skills`.
* resource/partiteai_task_definition: `ui_forms` gives a task UIs a client can
  show to complete it, keyed by form type. The only type is `A2UI`, whose
  definition is JSON. A change replaces all of the task's forms.
* data-source/partiteai_task_definition: `ui_forms`.
* **New Data Source:** `partiteai_a2ui_basic_form` builds an A2UI form for a
  task definition's `ui_forms` from the basic catalog's components, with a
  block for each component type. A property that can be bound or computed has
  `_path` and `_call` forms beside it, such as `text_path`.
* **New Data Source:** `partiteai_a2ui_form` builds an A2UI form from
  components given as a type and a JSON object of properties, from the basic
  catalog or a `catalog` of your own.
  Both check the form against its catalog at plan time.
* resource/partiteai_model_provider: `fireworks` (`api_key_wo`, optional
  `account_id`) and `cerebras` (`api_key_wo`) provider types.
* data-source/partiteai_model_provider: `fireworks` (`api_key_set`,
  `account_id`) and `cerebras` (`api_key_set`). Before this, a provider of
  either type read back with every block empty.
* resource/partiteai_model_profile, resource/partiteai_embedding_profile:
  `probe`, a write-only flag. The server sends one request to the provider
  before each create or update and refuses a profile the provider rejects;
  set `probe = false` to write a profile for a model the provider does not
  serve yet. Defaults to true, as before.
* resource/partiteai_model_provider: `probe`, a write-only flag. The server
  lists the provider's models with the credentials before a create, or an
  update that sends new credentials, and refuses credentials the provider
  rejects; set `probe = false` to write credentials it can't check yet. It
  does not cover the provider's profiles. Defaults to true, as before. Needs a
  server with `probe` on `createModelProvider` and `updateModelProvider`.
* resource/partiteai_task_definition: `overwrite_existing`, a write-only flag.
  When it is true and a task definition with the `name` already exists, create
  overwrites that one instead of failing, so a configuration can be applied to
  an organization that already has it, such as the task definitions every
  organization starts with. Needs a server with `overwriteExisting` on
  `createTaskDefinition`.
* **New Data Source:** `partiteai_judge_metric`, found by `id` or exact `name`.
* **New Data Source:** `partiteai_computed_metric`, found by `key`.
* resource/partiteai_mesh: `memory_store_id` attaches a memory store to the
  mesh. Every version of the mesh uses the same store. Removing it detaches
  the store and keeps its records.
* data-source/partiteai_mesh: `memory_store_id`.
