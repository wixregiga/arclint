// functions/no-orphaned-defs.js

export default function (defs, options, context) {
  if (!defs || typeof defs !== 'object' || Array.isArray(defs)) return [];

  const defKeys = Object.keys(defs);
  if (defKeys.length === 0) return [];

  // Search entire document raw AST data for references to #/$defs/<key>
  const docStr = JSON.stringify(context.document?.data || {});
  const errors = [];

  for (const key of defKeys) {
    const escapedKey = key.replace(/[.*+?^${}()|[\]\\]/g, '\\$&');
    const refRegex = new RegExp(`"\\$ref"\\s*:\\s*"#(?:\\/\\$defs|\\/definitions)\\/${escapedKey}"`);
    if (!refRegex.test(docStr)) {
      errors.push({
        message: `Definition "$defs.${key}" is not referenced anywhere in the schema document`,
        path: [...context.path, key],
      });
    }
  }

  return errors;
}
