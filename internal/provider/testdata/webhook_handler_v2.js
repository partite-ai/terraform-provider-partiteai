// The same as webhook_handler.js, in a file with other content: answers 403
// to a request whose X-Token header is the handler's token secret, and 401 to
// any other.
export function handle(request, config, secrets) {
  const token = request.header("X-Token");
  if (token !== secrets.get("token")) {
    return { status: 401, message: "bad token" };
  }
  return { status: 403, message: "token accepted" };
}
