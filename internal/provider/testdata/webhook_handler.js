// Answers 403 to a request whose X-Token header is the handler's token secret,
// and 401 to any other. It refuses every request, so none starts a run.
export function handle(request, config, secrets) {
  if (request.header("X-Token") !== secrets.get("token")) {
    return { status: 401, message: "bad token" };
  }
  return { status: 403, message: "token accepted" };
}
