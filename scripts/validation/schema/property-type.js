// functions/property-type.js

export default function (target, options, context) {
  if (!target || typeof target !== 'object' || Array.isArray(target)) return [];

  // Skip properties defined with $ref or combinators (oneOf, anyOf, allOf)
  if (target.$ref || target.oneOf || target.anyOf || target.allOf) {
    return [];
  }

  // Check if type is defined
  if (!target.type || typeof target.type !== 'string' || target.type.trim() === '') {
    const propName = context.path[context.path.length - 1];
    return [
      {
        message: `Property "${propName}" at ${context.path.join('.')} must declare an explicit type keyword`,
        path: context.path,
      },
    ];
  }

  return [];
}
