// functions/one-of-ref-description-match.js

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

export default function (branch, options, context) {
  const errors = [];

  const ref = branch?.$ref;
  if (!ref || typeof ref !== 'string') {
    return errors;
  }

  // 1. Must have a sibling description
  if (!branch.description || typeof branch.description !== 'string' || branch.description.trim() === '') {
    errors.push({
      message: `oneOf branch referencing "${ref}" must define a sibling description for editor hover hints`,
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

    if (branch.description !== targetDesc) {
      errors.push({
        message: `Description on oneOf branch referencing "${ref}" does not match the description in ${ref}.\nExpected: "${targetDesc}"\nFound:    "${branch.description}"`,
        path: [...context.path, 'description'],
      });
    }
  }

  return errors;
}