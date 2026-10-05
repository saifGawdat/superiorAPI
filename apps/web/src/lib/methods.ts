import type { HttpMethod } from "./types";

// Full class names (not built from strings) so Tailwind can find them.
const METHOD_BG: Record<HttpMethod, string> = {
  GET: "bg-method-get",
  POST: "bg-method-post",
  PUT: "bg-method-put",
  PATCH: "bg-method-patch",
  DELETE: "bg-method-delete",
};

/** Background class for an HTTP method; any other method gets slate. */
export function methodBg(method: string): string {
  return METHOD_BG[method as HttpMethod] ?? "bg-method-other";
}
