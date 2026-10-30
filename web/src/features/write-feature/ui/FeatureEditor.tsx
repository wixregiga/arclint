import { useEffect, useImperativeHandle, useRef, type Ref } from "react";
import { createFeatureEditorView, type FeatureEditorHandle, type FeatureEditorSettings, type FeatureEditorView } from "../lib/editor-view";

export interface FeatureEditorProps extends FeatureEditorSettings {
  ref?: Ref<FeatureEditorHandle>;
}

/**
 * Hosts one feature's CodeMirror view, created once for the mount's
 * lifetime; later props reach it through a ref, and a changed language
 * is pushed into it. Adapted from BWrite's feature-editor
 * (src/contexts/authoring/ui/feature-editor.tsx, MPL-2.0, commit c75ead5).
 */
export function FeatureEditor(props: FeatureEditorProps) {
  const hostRef = useRef<HTMLDivElement>(null);
  const viewRef = useRef<FeatureEditorView | null>(null);
  const latest = useRef(props);
  latest.current = props;

  useEffect(() => {
    const view = createFeatureEditorView(hostRef.current!, () => latest.current);
    viewRef.current = view;
    return () => {
      view.destroy();
      viewRef.current = null;
    };
  }, []);

  useEffect(() => {
    viewRef.current?.setLanguage(props.entries);
  }, [props.entries]);

  useImperativeHandle(
    props.ref,
    () => ({
      focus: () => viewRef.current?.focus(),
      select: (from, to) => viewRef.current?.select(from, to),
      inspectAtCursor: () => viewRef.current?.inspectAtCursor() ?? false,
      openInsertion: () => viewRef.current?.openInsertion(),
      associate: (from, to, association) => viewRef.current?.associate(from, to, association),
      forget: (id) => viewRef.current?.forget(id),
    }),
    [],
  );

  return <div ref={hostRef} className="st-editor-host" data-testid="feature-editor" />;
}
