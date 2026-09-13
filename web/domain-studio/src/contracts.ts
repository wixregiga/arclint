import type { FocusView } from './view-state';

export type ConceptKind = 'unclassified' | 'aggregate' | 'entity' | 'value_object' | 'domain_event' | 'domain_service' | 'specification' | 'repository' | 'factory';
export interface DomainContext { id: string; name: string; description: string; color: string; position: [number, number, number] }
export interface Concept { id: string; name: string; kind: ConceptKind; contextId: string; definition: string; position: [number, number, number]; invariants: string[]; identity?: string; ownerId?: string; aliases?: string[] }
export interface Relationship { id: string; source: string; target: string; label: string }
export interface DomainProject { version: 1; name: string; description: string; contexts: DomainContext[]; concepts: Concept[]; relationships: Relationship[]; sourceDocument?: Record<string, unknown> }
export interface Baseline { name: string; capturedAt: string; project: DomainProject }
export interface Finding { id: string; severity: 'warning' | 'error'; subjectId: string; title: string; message: string }
export interface Change { id: string; type: 'added' | 'changed' | 'removed'; subject: 'concept' | 'context' | 'relationship'; name: string; detail: string }
export interface PatternSummary { name: string; description: string; rules: { id: string; description: string }[] }
export type StudioMode = 'domain' | 'patterns' | 'baseline';
export interface SceneState { project: DomainProject; selectedId: string | null; mode: StudioMode; baseline: Baseline | null; findings: Finding[]; search: string; scopeId?: string | null; aggregateId?: string | null; view?: FocusView; lens?: 'meaning' | 'governance' }
export interface SceneController { update(state: SceneState): void; focus(id: string): void; overview(): void; topView(): void; zoom(direction: number): void; dispose(): void; screenPosition?(id: string): {x: number; y: number} | null }
export interface SceneCallbacks { select(id: string | null): void; move(id: string, position: [number, number, number]): void; enter?(id: string): void; keeper?(): void }
