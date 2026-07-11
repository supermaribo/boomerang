/** Coerce API list fields — Go nil slices serialize as JSON null. */
export function asArray<T>(value: T[] | null | undefined): T[] {
  return Array.isArray(value) ? value : [];
}
