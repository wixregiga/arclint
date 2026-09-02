// functions/description-formatting.js

export default function (target, options, context) {
  if (typeof target !== 'string' || target.trim() === '') return [];

  const errors = [];
  const text = target.trim();

  // 1. Initial Capitalization / Symbol
  if (/^[a-z]/.test(text)) {
    errors.push({
      message: `Description must start with an uppercase letter, code identifier, or quote: "${text.slice(0, 25)}..."`,
      path: context.path,
    });
  }

  // 2. Terminal Punctuation
  if (!/[.!?:]$/.test(text)) {
    errors.push({
      message: `Description must end with terminal punctuation ('.', '!', '?', or ':'): "...${text.slice(-20)}"`,
      path: context.path,
    });
  }

  // 3. Glitched Punctuation Sequences (e.g., "-.", ",.", "..")
  if (/[-_,;]\./.test(text)) {
    errors.push({
      message: `Description contains glitched punctuation sequence ("${text.match(/[-_,;]\./)[0]}")`,
      path: context.path,
    });
  }

  // 4. Paragraph and List Formatting
  if (text.includes('\n')) {
    // Excessive consecutive blank lines (3+ newlines)
    if (/\n{3,}/.test(text)) {
      errors.push({
        message: `Description contains excessive consecutive blank lines`,
        path: context.path,
      });
    }

    // Markdown list items (- item or * item) must be preceded by a blank line (\n\n)
    if (/[^\n]\n[*-]\s/.test(text)) {
      errors.push({
        message: `Markdown list items must be preceded by a blank line (\\n\\n) for proper formatting`,
        path: context.path,
      });
    }
  }

  // 5. Run-on Sentence Detection
  const sentences = text.split(/(?<=[.!?])\s+/);
  for (const sentence of sentences) {
    const words = sentence.trim().split(/\s+/).filter(Boolean);
    if (words.length > 45 && sentence.length > 250) {
      errors.push({
        message: `Description contains a run-on sentence (${words.length} words, ${sentence.length} characters): "${sentence.slice(0, 60)}..."`,
        path: context.path,
      });
    }
  }

  return errors;
}
