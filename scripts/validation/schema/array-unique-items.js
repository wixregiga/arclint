// functions/array-unique-items.js

export default function (node, options, context) {
  if (!node || typeof node !== 'object' || Array.isArray(node)) return [];

  // Only check schemas declaring type: "array"
  if (node.type !== 'array') return [];

  // If uniqueItems is already true, it passes
  if (node.uniqueItems === true) return [];

  // If options.skipEnumItems is true, skip arrays whose items define enum
  // to avoid duplicating enum-arrays-must-set-unique-items
  if (options?.skipEnumItems && node.items && typeof node.items === 'object' && node.items.enum) {
    return [];
  }

  const errors = [];
  errors.push({
    message: `Array schema at "${context.path.join('.')}" must declare uniqueItems: true to prevent duplicate entries (doubles)`,
    path: [...context.path, 'uniqueItems'],
  });

  return errors;
}
