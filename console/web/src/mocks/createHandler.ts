import { HttpResponse, delay as mswDelay } from "msw";
import { z } from "zod/v4";
import type { ZodTypeAny } from "zod/v4";

export interface ApiResponse<T> {
  code: number;
  data: T;
  message: string;
}

export async function withDelay(ms: number = 200): Promise<void> {
  await mswDelay(ms);
}

export function successResponse<T>(data: T, message: string = "ok") {
  return HttpResponse.json({
    code: 0,
    data,
    message,
  });
}

/** Parse `data` with Zod before wrapping — catches mock ↔ contract drift early. */
export function successWithSchema<T extends ZodTypeAny>(
  schema: T,
  data: unknown,
  message: string = "ok",
) {
  const parsed = schema.parse(data);
  return successResponse(parsed, message);
}

export function paginatedListSchema<T extends ZodTypeAny>(itemSchema: T) {
  return z.object({
    list: z.array(itemSchema),
    total: z.number().int().nonnegative(),
  });
}

export function paginatedWithSchema<T extends ZodTypeAny>(
  itemSchema: T,
  data: unknown,
  message: string = "ok",
) {
  return successWithSchema(paginatedListSchema(itemSchema), data, message);
}

const NullBodySchema = z.null();

export function successWithNullBody(message: string = "ok") {
  return successWithSchema(NullBodySchema, null, message);
}

export function errorResponse(code: number, message: string, data: unknown = null) {
  return HttpResponse.json({
    code,
    data,
    message,
  });
}

export const ERROR_CODES = {
  INVALID_CREDENTIALS: 1001,
  NOT_FOUND: 1002,
  UNAUTHORIZED: 1003,
  FORBIDDEN: 1004,
  BAD_REQUEST: 1005,
  INTERNAL_ERROR: 1006,
} as const;
