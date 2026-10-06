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

FEATURES:

* **New Resource:** `partiteai_skill`. A skill is a folder of text files, given
  as `files` by path, `SKILL.md` among them. Its own `name` is unique; the name
  `SKILL.md` declares is what agents see.
* **New Data Source:** `partiteai_skill`, found by `id` or `name`.
* resource/partiteai_agent: `skills` gives an agent skills to load. No two of an
  agent's skills can declare the same name.
* data-source/partiteai_agent: `skills`.
* **New Data Source:** `partiteai_judge_metric`, found by `id` or exact `name`.
* **New Data Source:** `partiteai_computed_metric`, found by `key`.
