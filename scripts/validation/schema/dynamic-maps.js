// functions/dynamic-maps.js

export default function (target, options, context) {
  if (!target || typeof target !== 'object' || Array.isArray(target)) return [];

  // If additionalProperties is a subschema object, propertyNames should be defined
  if (
    target.type === 'object' &&
    target.additionalProperties &&
    typeof target.additionalProperties === 'object' &&
    !Array.isArray(target.additionalProperties)
  ) {
    if (!target.propertyNames) {
      return [
        {
          message: `Dynamic map at ${context.path.join('.')} should define propertyNames to constrain dictionary keys`,
          path: context.path,
        },
      ];
    }
  }

  return [];
}
