// functions/required-no-default.js

export default function (target, options, context) {
  if (!target || typeof target !== 'object' || Array.isArray(target)) return [];
  if (!Array.isArray(target.required) || !target.properties || typeof target.properties !== 'object') {
    return [];
  }

  const errors = [];
  for (const reqProp of target.required) {
    const propSchema = target.properties[reqProp];
    if (propSchema && typeof propSchema === 'object' && propSchema.default !== undefined) {
      errors.push({
        message: `Required property "${reqProp}" specifies a default value (${JSON.stringify(propSchema.default)}), creating ambiguous optional/required expectations`,
        path: [...context.path, 'properties', reqProp, 'default'],
      });
    }
  }

  return errors;
}
