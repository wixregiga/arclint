// functions/properties-object.js

export default function (target, options, context) {
  if (!target || typeof target !== 'object' || Array.isArray(target)) return [];

  // If target defines properties, it must declare type: "object"
  if (target.properties && typeof target.properties === 'object' && !Array.isArray(target.properties)) {
    if (target.type !== 'object') {
      return [
        {
          message: `Schema declaring properties at ${context.path.join('.')} lacks explicit type: object. Advisory: Agents must not fix this without properly investigating the issue, presenting evidence for or against fixing to a human, and waiting for that choice to be made by the human before working on it.`,
          path: context.path,
        },
      ];
    }
  }

  return [];
}
