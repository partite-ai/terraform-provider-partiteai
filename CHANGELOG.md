## 0.1.0 (Unreleased)

BREAKING CHANGES:

* resource/partiteai_mcp_connection: `stdio_transport` is removed. Mesh no
  longer runs MCP servers as child processes, so there is nothing behind it.
  Replace a stdio connection with `tunnel_transport`, and run the server
  yourself with `partite-mcp-relay`, which launches it over stdio the same way
  and reaches mesh over an outbound tunnel.
* data-source/partiteai_mcp_connection: `stdio_transport` is removed, for the
  same reason.

FEATURES:
