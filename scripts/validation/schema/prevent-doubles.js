// functions/prevent-doubles.js
export default function (target, options, context) {
  if (!Array.isArray(target)) return [];
  const errors = [];
  const seen = new Set();

  for (let i = 0; i < target.length; i++) {
    const item = target[i];
    const key = typeof item === 'object' && item !== null ? JSON.stringify(item) : `${typeof item}:${item}`;
    if (seen.has(key)) {
      errors.push({
        message: `Array contains duplicate element (double): ${typeof item === 'string' ? `"${item}"` : key}`,
        path: [...context.path, i],
      });
    } else {
      seen.add(key);
    }
  }

  return errors;
}
