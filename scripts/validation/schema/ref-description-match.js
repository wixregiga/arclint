// functions/ref-description-match.js

function resolvePointer(doc, ref) {
  if (!ref || !ref.startsWith('#/')) return undefined;
  const segments = ref.slice(2).split('/').map((seg) => seg.replace(/~1/g, '/').replace(/~0/g, '~'));
  let curr = doc;
  for (const seg of segments) {
    if (curr == null || typeof curr !== 'object') return undefined;
    curr = curr[seg];
  }
  return curr;
}

export default function (node, options, context) {
  if (!node || typeof node !== 'object' || Array.isArray(node)) return [];
  const ref = node.$ref;
  if (!ref || typeof ref !== 'string') return [];

  // If skipOneOf is enabled, skip nodes that are direct elements of oneOf
  // because one-of-ref-must-match-target-description handles them
  if (options?.skipOneOf && context.path.length >= 2 && context.path[context.path.length - 2] === 'oneOf') {
    return [];
  }

  const errors = [];

  // 1. Must define a sibling description
  if (!node.description || typeof node.description !== 'string' || node.description.trim() === '') {
    errors.push({
      message: `Reference "${ref}" must define a sibling description matching the target definition's description`,
      path: [...context.path, 'description'],
    });
    return errors;
  }

  // 2. Resolve the target in document and verify description equality
  if (ref.startsWith('#/')) {
    const targetDef = resolvePointer(context.document?.data, ref);

    if (!targetDef) {
      errors.push({
        message: `Referenced definition "${ref}" was not found in schema`,
        path: [...context.path, '$ref'],
      });
      return errors;
    }

    const targetDesc = targetDef.description;
    if (!targetDesc || typeof targetDesc !== 'string' || targetDesc.trim() === '') {
      errors.push({
        message: `Referenced definition "${ref}" has no description defined`,
        path: [...context.path, '$ref'],
      });
      return errors;
    }

    if (node.description !== targetDesc) {
      errors.push({
        message: `Sibling description on "${ref}" does not match target definition's description.\nExpected: "${targetDesc}"\nFound:    "${node.description}"`,
        path: [...context.path, 'description'],
      });
    }
  }

  return errors;
}
