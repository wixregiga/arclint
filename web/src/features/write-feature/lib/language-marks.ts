/*
 * The language drawn in the prose: every use the text holds, decorated
 * by kind, with the concept's badge immediately left of a tied use and a
 * hover card giving the current definition. Adapted from BWrite's
 * domain-terms (src/contexts/authoring/editor/domain-terms.ts, MPL-2.0,
 * commit c75ead5). A cue is drawn quietly; it is a cue, not a claim.
 */
import { Prec, StateEffect, StateField, type EditorState, type Extension, type Range } from "@codemirror/state";
import { Decoration, EditorView, WidgetType, activateHover, closeHoverTooltips, hasHoverTooltips, hoverTooltip, keymap, type DecorationSet, type TooltipView } from "@codemirror/view";
import { CONCEPT_MARK, CONCEPT_TITLE, describeReading, readingOf, readingWord, useAt, usesOf, type Entry, type Use } from "../../../entities/language/index";
import { annotationsField } from "./annotations";

export const setLanguage = StateEffect.define<readonly Entry[]>();

interface LanguageState {
  entries: readonly Entry[];
  uses: readonly Use[];
  decorations: DecorationSet;
}

/** The badge left of a use: the concept's mark, and for a value or a rule reading a word, so the reading is not told by colour alone. */
class UseMarkWidget extends WidgetType {
  constructor(
    readonly tone: string,
    readonly mark: string,
    readonly label: string,
    readonly word: string | null,
  ) {
    super();
  }

  override eq(other: UseMarkWidget) {
    return other.tone === this.tone && other.mark === this.mark && other.label === this.label && other.word === this.word;
  }

  toDOM(view: EditorView) {
    const doc = view.dom.ownerDocument;
    const wrap = doc.createElement("span");
    wrap.className = "st-use-mark";
    const badge = doc.createElement("span");
    badge.className = `st-kind-badge st-kind-badge--${this.tone} st-kind-badge--small`;
    badge.setAttribute("role", "img");
    badge.setAttribute("aria-label", this.label);
    badge.title = this.label;
    badge.textContent = this.mark;
    wrap.append(badge);
    if (this.word !== null) {
      const word = doc.createElement("span");
      word.className = "st-use-mark__word";
      word.textContent = this.word;
      wrap.append(word);
    }
    return wrap;
  }

  override ignoreEvent() {
    return false;
  }
}

function decorationsFor(use: Use): Range<Decoration>[] {
  if (!use.placed || use.from >= use.to) return [];
  const tone = use.entry?.concept ?? "question";
  const attributes: Record<string, string> = { "data-use": use.kind };
  if (use.entry) attributes["data-entry"] = use.entry.key;
  if (use.reference) attributes["data-reference"] = use.reference.id;
  const highlight = Decoration.mark({ class: `st-use st-use--${use.kind} st-use--${tone}`, attributes }).range(use.from, use.to);
  switch (use.kind) {
    case "tied": {
      const entry = use.entry!;
      const word = use.reading ? readingWord(use.reading) : null;
      const widget = new UseMarkWidget(entry.concept, CONCEPT_MARK[entry.concept], `${CONCEPT_TITLE[entry.concept]} ${entry.name}`, word);
      return [Decoration.widget({ widget, side: -1 }).range(use.from), highlight];
    }
    case "unresolved":
      return [Decoration.widget({ widget: new UseMarkWidget("question", "!", "Unresolved annotation", null), side: -1 }).range(use.from), highlight];
    default:
      return [highlight];
  }
}

function compute(state: EditorState, entries: readonly Entry[]): LanguageState {
  const uses = usesOf(state.doc.toString(), state.field(annotationsField), entries);
  return { entries, uses, decorations: Decoration.set(uses.flatMap(decorationsFor), true) };
}

export const languageField = StateField.define<LanguageState>({
  create: () => ({ entries: [], uses: [], decorations: Decoration.none }),
  update(value, tr) {
    let entries = value.entries;
    for (const effect of tr.effects) if (effect.is(setLanguage)) entries = effect.value;
    const annotationsChanged = tr.startState.field(annotationsField) !== tr.state.field(annotationsField);
    if (!tr.docChanged && entries === value.entries && !annotationsChanged) return value;
    return compute(tr.state, entries);
  },
  provide: (field) => EditorView.decorations.from(field, (v) => v.decorations),
});

export function usesIn(state: EditorState): readonly Use[] {
  return state.field(languageField).uses;
}

function element<K extends keyof HTMLElementTagNameMap>(doc: Document, tag: K, className: string, text?: string) {
  const el = doc.createElement(tag);
  el.className = className;
  if (text !== undefined) el.textContent = text;
  return el;
}

function whereOf(entry: Entry): string {
  if (entry.concept === "bounded_context") return "bounded context";
  return entry.owner === "" ? `${CONCEPT_TITLE[entry.concept]} in ${entry.context}` : `${CONCEPT_TITLE[entry.concept]} of ${entry.owner} in ${entry.context}`;
}

/** The hover card: what the text names, where it is recorded and what it means now. */
function createCard(view: EditorView, use: Use): TooltipView {
  const doc = view.dom.ownerDocument;
  const dom = element(doc, "div", "st-use-card");
  dom.setAttribute("role", "note");
  switch (use.kind) {
    case "cue":
    case "tied": {
      const entry = use.entry!;
      dom.setAttribute("aria-label", `${use.kind === "cue" ? "Recognised" : "Tied to"} ${CONCEPT_TITLE[entry.concept]} ${entry.name}`);
      dom.append(element(doc, "div", "st-use-card__eyebrow", use.kind === "cue" ? "Recognised, not confirmed" : "Tied on purpose"));
      dom.append(element(doc, "div", "st-use-card__name", entry.name));
      dom.append(element(doc, "div", "st-use-card__where", whereOf(entry)));
      dom.append(element(doc, "p", "st-use-card__reading", readingOf(entry)));
      if (use.reading && use.reading !== "names") dom.append(element(doc, "p", "st-use-card__note", describeReading(use.reading, entry.name)));
      break;
    }
    case "ambiguous":
      dom.setAttribute("aria-label", `Ambiguous ${use.text}`);
      dom.append(element(doc, "div", "st-use-card__eyebrow", "Ambiguous"));
      dom.append(element(doc, "div", "st-use-card__name", use.text));
      dom.append(element(doc, "p", "st-use-card__note", `Recorded in ${use.candidates.map((c) => `${c.context} (${CONCEPT_TITLE[c.concept].toLowerCase()})`).join(" and ")}. Select it and tie it to the one you mean.`));
      break;
    case "plain":
      dom.setAttribute("aria-label", `Plain ${use.text}`);
      dom.append(element(doc, "div", "st-use-card__eyebrow", "Plain word"));
      dom.append(element(doc, "p", "st-use-card__note", "Marked as not a use of the language."));
      break;
    case "unresolved":
      dom.setAttribute("aria-label", `Unresolved ${use.text}`);
      dom.append(element(doc, "div", "st-use-card__eyebrow", "Unresolved"));
      dom.append(element(doc, "p", "st-use-card__note", use.reason ?? ""));
      break;
  }
  return { dom };
}

export interface LanguageMarksSupport {
  extension: Extension;
  /** Opens the card for the use at the cursor; false when there is none. */
  inspectAtCursor(view: EditorView): boolean;
}

export function languageMarks(initial: readonly Entry[]): LanguageMarksSupport {
  const hover = hoverTooltip(
    (view, pos, side) => {
      const use = useAt(usesIn(view.state), pos);
      if (!use) return null;
      if ((pos === use.from && side < 0) || (pos === use.to && side > 0)) return null;
      return { pos: use.from, end: use.to, above: true, create: (v) => createCard(v, use) };
    },
    { hideOnChange: true, hoverTime: 250 },
  );

  function inspectAtCursor(view: EditorView): boolean {
    const use = useAt(usesIn(view.state), view.state.selection.main.head);
    if (!use) return false;
    activateHover(view, use.from, 1, { tooltip: hover, until: (tr) => tr.docChanged || tr.selection !== undefined });
    return true;
  }

  const extension: Extension = [
    languageField.init((state) => compute(state, initial)),
    hover,
    Prec.high(
      keymap.of([
        {
          key: "Escape",
          run(view) {
            if (!hasHoverTooltips(view.state)) return false;
            view.dispatch({ effects: closeHoverTooltips });
            return true;
          },
        },
      ]),
    ),
  ];
  return { extension, inspectAtCursor };
}
