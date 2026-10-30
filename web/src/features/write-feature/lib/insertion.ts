/*
 * Inserting a recorded name at the cursor. Ctrl-Space opens a list of
 * the recorded entries, narrowed by the word being typed; choosing one
 * writes its name and ties the occurrence to it, so an inserted name is
 * never ambiguous. Adapted from BWrite's term-insertion
 * (src/contexts/authoring/editor/term-insertion.ts, MPL-2.0, commit
 * c75ead5), listing the context beside each name because the same
 * spelling may be recorded in several.
 */
import { Prec, StateEffect, StateField, type EditorState, type Extension, type TransactionSpec } from "@codemirror/state";
import { EditorView, keymap, showTooltip, type Tooltip } from "@codemirror/view";
import { CONCEPT_MARK, CONCEPT_TITLE, normalizeName, RECOGNIZED_CONCEPTS, termOf, type Entry } from "../../../entities/language/index";
import { annotateTransaction } from "./annotations";
import { newReferenceId } from "./reference-id";

const WORD = /[\p{L}\p{M}\p{N}_]/u;

export interface InsertionMatch {
  from: number;
  to: number;
  query: string;
  entries: readonly Entry[];
}

function escapeRegExp(text: string): string {
  return text.replace(/[.*+?^${}()|[\]\\]/g, "\\$&");
}

/** The entries a name can be inserted for: those recognition names, in the recording's order. */
export function insertable(entries: readonly Entry[]): Entry[] {
  return entries.filter((e) => RECOGNIZED_CONCEPTS.includes(e.concept));
}

/**
 * The longest same-line run beginning at a word boundary immediately
 * before the cursor that begins some recorded name, with the entries it
 * begins. Whitespace or a line start before the cursor opens the whole
 * language.
 */
export function insertionMatch(text: string, cursor: number, entries: readonly Entry[]): InsertionMatch {
  const start = text.lastIndexOf("\n", cursor - 1) + 1;
  const left = text.slice(start, cursor);
  let from = cursor;
  let query = "";
  let candidates: readonly Entry[] = entries;
  if (left && !/\s$/u.test(left)) {
    candidates = [];
    const points = [...left];
    let offset = 0;
    for (let i = 0; i < points.length; i++) {
      const previous = points[i - 1];
      if (WORD.test(points[i]) && (!previous || !WORD.test(previous))) {
        const run = left.slice(offset);
        const pattern = new RegExp(`^${escapeRegExp(normalizeName(run))}`, "iu");
        const matches = entries.filter((e) => pattern.test(normalizeName(e.name)));
        if (matches.length) {
          from = start + offset;
          query = run;
          candidates = matches;
          break;
        }
      }
      offset += points[i].length;
    }
    if (!candidates.length && WORD.test(points.at(-1)!)) {
      const word = left.match(/[\p{L}\p{M}\p{N}_]+$/u)![0];
      query = word;
      from = cursor - word.length;
    } else if (!candidates.length) {
      candidates = entries;
    }
  }
  const exact = new RegExp(`^${escapeRegExp(normalizeName(query))}$`, "iu");
  const sorted = [...candidates].sort((a, b) => Number(exact.test(normalizeName(b.name))) - Number(exact.test(normalizeName(a.name))) || a.name.localeCompare(b.name) || a.context.localeCompare(b.context));
  return { from, to: cursor, query, entries: sorted };
}

/** Writes the entry's name over the match and ties it; the cursor lands after the name. */
export function insertTransaction(state: EditorState, match: InsertionMatch, entry: Entry): TransactionSpec {
  const spec = annotateTransaction(state, match.from, match.to, { id: newReferenceId(), reading: "names", term: termOf(entry) }, entry.name);
  return { ...spec, selection: { anchor: match.from + entry.name.length } };
}

interface Menu extends InsertionMatch {
  active: number;
}

const setMenu = StateEffect.define<Menu | null>();
let nextMenuId = 0;

export interface InsertionSupport {
  extension: Extension;
  open(view: EditorView): boolean;
}

export function insertion(getEntries: () => readonly Entry[]): InsertionSupport {
  const menuId = `st-insert-${++nextMenuId}`;
  const field = StateField.define<Menu | null>({
    create: () => null,
    update(value, tr) {
      for (const effect of tr.effects) if (effect.is(setMenu)) return effect.value;
      return tr.docChanged || tr.selection !== undefined || tr.effects.length > 0 ? null : value;
    },
    provide: (f) => showTooltip.from(f, (menu): Tooltip | null => (menu ? { pos: menu.to, above: false, create: createTooltip } : null)),
  });

  function createTooltip(view: EditorView) {
    const doc = view.dom.ownerDocument;
    const dom = doc.createElement("div");
    dom.className = "st-insert";
    dom.id = menuId;
    dom.setAttribute("role", "listbox");
    dom.setAttribute("aria-label", "Insert a recorded name");
    let rendered: Menu | null = null;
    const render = () => {
      const current = view.state.field(field);
      if (!current || current === rendered) return;
      if (current.entries === rendered?.entries) {
        Array.from(dom.children).forEach((row, i) => row.setAttribute("aria-selected", String(i === current.active)));
        rendered = current;
        return;
      }
      rendered = current;
      dom.replaceChildren();
      if (!current.entries.length) {
        const empty = doc.createElement("div");
        empty.className = "st-insert__empty";
        empty.setAttribute("role", "status");
        empty.textContent = current.query ? `Nothing recorded begins with “${current.query}”` : "Nothing is recorded yet";
        dom.append(empty);
      }
      current.entries.forEach((entry, i) => {
        const row = doc.createElement("div");
        row.id = `${dom.id}-${i}`;
        row.className = "st-insert__option";
        row.setAttribute("role", "option");
        row.setAttribute("aria-selected", String(i === current.active));
        const badge = doc.createElement("span");
        badge.className = `st-kind-badge st-kind-badge--${entry.concept} st-kind-badge--small`;
        badge.setAttribute("aria-label", CONCEPT_TITLE[entry.concept]);
        badge.textContent = CONCEPT_MARK[entry.concept];
        const name = doc.createElement("span");
        name.className = "st-insert__name";
        name.textContent = entry.name;
        const where = doc.createElement("small");
        where.className = "st-insert__where";
        where.textContent = entry.concept === "bounded_context" ? "bounded context" : entry.owner ? `${entry.context} · ${entry.owner}` : entry.context;
        row.append(badge, name, where);
        row.addEventListener("mousedown", (event) => event.preventDefault());
        row.addEventListener("click", () => choose(view, i));
        dom.append(row);
      });
    };
    render();
    return {
      dom,
      update: render,
      positioned() {
        const row = dom.children[view.state.field(field)?.active ?? 0] as HTMLElement | undefined;
        if (!row) return;
        const top = row.offsetTop;
        const bottom = top + row.offsetHeight;
        if (top < dom.scrollTop) dom.scrollTop = top;
        else if (bottom > dom.scrollTop + dom.clientHeight) dom.scrollTop = bottom - dom.clientHeight;
      },
    };
  }

  const close = (view: EditorView) => {
    if (!view.state.field(field)) return false;
    view.dispatch({ effects: setMenu.of(null) });
    return true;
  };
  const choose = (view: EditorView, index?: number) => {
    const menu = view.state.field(field);
    if (!menu) return false;
    const entry = menu.entries[index ?? menu.active];
    if (entry) view.dispatch(insertTransaction(view.state, menu, entry));
    else close(view);
    view.focus();
    return true;
  };
  const move = (view: EditorView, delta: number) => {
    const menu = view.state.field(field);
    if (!menu) return false;
    if (menu.entries.length) view.dispatch({ effects: setMenu.of({ ...menu, active: (menu.active + delta + menu.entries.length) % menu.entries.length }) });
    return true;
  };
  const open = (view: EditorView) => {
    const cursor = view.state.selection.main.head;
    view.dispatch({ effects: setMenu.of({ ...insertionMatch(view.state.doc.toString(), cursor, insertable(getEntries())), active: 0 }) });
    return true;
  };

  return {
    open,
    extension: [
      field,
      EditorView.contentAttributes.compute([field], (state) => {
        const menu = state.field(field);
        if (!menu) return {};
        return {
          role: "combobox",
          "aria-autocomplete": "list",
          "aria-controls": menuId,
          "aria-haspopup": "listbox",
          "aria-expanded": "true",
          ...(menu.entries.length ? { "aria-activedescendant": `${menuId}-${menu.active}` } : {}),
        };
      }),
      Prec.highest(
        keymap.of([
          { key: "Ctrl-Space", run: open, preventDefault: true },
          { key: "ArrowDown", run: (view) => move(view, 1) },
          { key: "ArrowUp", run: (view) => move(view, -1) },
          { key: "Enter", run: choose },
          { key: "Tab", run: choose },
          { key: "Escape", run: close },
        ]),
      ),
      EditorView.domEventHandlers({ blur: (_event, view) => close(view) }),
    ],
  };
}
