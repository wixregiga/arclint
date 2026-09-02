// functions/no-whitespace-drift.js
export default function (target, options, context) {
  if (typeof target !== 'string') return [];

  const errors = [];

  // 1. Check for non-breaking spaces or non-standard whitespace
  if (/[\u00A0\u2000-\u200B\u202F\u205F]/.test(target)) {
    errors.push({
      message: `${context.path[context.path.length - 1]} contains illegal non-breaking or unicode spaces (use standard ASCII space)`,
      path: context.path,
    });
  }

  // 2. Check for overall leading/trailing whitespace
  if (target !== target.trim()) {
    errors.push({
      message: `${context.path[context.path.length - 1]} has leading or trailing whitespace`,
      path: context.path,
    });
  }

  // 3. Check for 2+ consecutive horizontal spaces (standard or non-breaking)
  if (/[ \u00A0]{2,}/.test(target)) {
    errors.push({
      message: `${context.path[context.path.length - 1]} contains two or more consecutive spaces`,
      path: context.path,
    });
  }

  // 4. Check for trailing spaces before a newline
  if (/[ \u00A0]+\r?\n/.test(target)) {
    errors.push({
      message: `${context.path[context.path.length - 1]} contains trailing spaces before a newline`,
      path: context.path,
    });
  }

  return errors;
}