// functions/property-description.js

export default function (target, options, context) {
  if (!target || typeof target !== 'object' || Array.isArray(target)) return [];

  // Skip condition branches inside anyOf, oneOf, allOf
  if (context.path.includes('anyOf') || context.path.includes('oneOf') || context.path.includes('allOf')) {
    return [];
  }

  // If target has $ref, ref-requires-description enforces sibling description at error severity
  if (options?.skipRefs && typeof target.$ref === 'string') {
    return [];
  }

  // Check if description is missing or empty
  if (!target.description || typeof target.description !== 'string' || target.description.trim() === '') {
    const propName = context.path[context.path.length - 1];
    return [
      {
        message: `Property "${propName}" should define a description for editor hover hints and documentation`,
        path: context.path,
      },
    ];
  }

  return [];
}
