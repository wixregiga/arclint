// functions/combinators.js

export default function (target, options, context) {
  if (!Array.isArray(target)) return [];

  const min = typeof options?.min === 'number' ? options.min : 2;
  if (target.length < min) {
    const key = context.path[context.path.length - 1];
    return [
      {
        message: `Combinator "${key}" must contain at least ${min} subschemas (found ${target.length})`,
        path: context.path,
      },
    ];
  }

  return [];
}
