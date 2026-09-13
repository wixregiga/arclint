import * as THREE from 'three';
import { OrbitControls } from 'three/addons/controls/OrbitControls.js';
import { CSS2DObject, CSS2DRenderer } from 'three/addons/renderers/CSS2DRenderer.js';
import type { Concept, DomainContext, DomainProject, SceneCallbacks, SceneController, SceneState } from './contracts';
import { conceptContracts, relationshipDescription } from './model-evidence';
import { projectView, type FocusView } from './view-state';
import './scene.css';

const CHALK = 0xe9eff1, GRAPHITE = 0x233e4b, STONE = 0xb9cecf, ACCENT = 0xb68032;
const CONTEXT = 0x3e7774;
type Subject = Concept | DomainContext;
type Flight = { from: THREE.Vector3; fromTarget: THREE.Vector3; to: THREE.Vector3; target: THREE.Vector3; started: number };

/** This stage displays only the subjects allocated by FocusView. It never constructs domain hierarchy. */
export function createScene(container: HTMLElement, callbacks: SceneCallbacks): SceneController {
  const host = document.createElement('div'); host.className = 'atlas-scene study-scene domain-regions'; container.append(host);
  let renderer: THREE.WebGLRenderer;
  try { renderer = new THREE.WebGLRenderer({ antialias: true, alpha: false, powerPreference: 'high-performance' }); }
  catch { return fallback(host, callbacks); }
  renderer.setPixelRatio(Math.min(devicePixelRatio || 1, 2)); renderer.setClearColor(CHALK, 1); renderer.outputColorSpace = THREE.SRGBColorSpace;
  renderer.toneMapping = THREE.ACESFilmicToneMapping; renderer.toneMappingExposure = 1.05;
  renderer.shadowMap.enabled = true; renderer.shadowMap.type = THREE.PCFSoftShadowMap; renderer.shadowMap.autoUpdate = false; renderer.shadowMap.needsUpdate = true;
  const canvas = renderer.domElement; canvas.tabIndex = 0; canvas.setAttribute('aria-label', 'Recorded domain map. Drag to orbit, right-drag to pan, scroll to zoom. Select a bounded context to enter its language. Shift-drag a domain entry to arrange its saved position.'); host.append(canvas);
  const leaders = document.createElementNS('http://www.w3.org/2000/svg', 'svg'); leaders.classList.add('study-leaders'); leaders.setAttribute('aria-hidden', 'true'); host.append(leaders);
  const labels = new CSS2DRenderer(); labels.domElement.className = 'study-labels'; host.append(labels.domElement);
  const scene = new THREE.Scene(); scene.background = new THREE.Color(CHALK);
  const camera = new THREE.PerspectiveCamera(38, 1, .1, 1400); camera.position.set(40, 64, 86);
  const controls = new OrbitControls(camera, canvas); controls.listenToKeyEvents(canvas);
  const reducedMotion = matchMedia('(prefers-reduced-motion: reduce)').matches;
  controls.enableDamping = !reducedMotion; controls.dampingFactor = .1; controls.minDistance = 10; controls.maxDistance = 800; controls.maxPolarAngle = Math.PI * .465;
  scene.add(new THREE.HemisphereLight(0xffffff, 0x929b8e, 1.7));
  const key = new THREE.DirectionalLight(0xfffbf0, 2.8); key.position.set(-36, 70, 42); key.castShadow = true;
  key.shadow.mapSize.set(2048, 2048); Object.assign(key.shadow.camera, { left: -150, right: 150, top: 150, bottom: -150, far: 300 }); key.shadow.normalBias = .055;
  key.shadow.radius = 4; scene.add(key);
  const floor = piece(new THREE.PlaneGeometry(1800, 1800), CHALK); floor.rotation.x = -Math.PI / 2; floor.position.y = -.35; floor.castShadow = false; scene.add(floor);
  let world = new THREE.Group(); scene.add(world);
  let state: SceneState | null = null, view: FocusView | null = null;
  let disposed = false, needsRender = true, frame = 0, initialFrame = true;
  let priorView = '', flight: Flight | null = null;
  const locations = new Map<string, THREE.Vector3>(), groups = new Map<string, THREE.Group>();
  const picks: THREE.Object3D[] = [], textObjects: CSS2DObject[] = [];
  const hoverLabels = new Map<string, HTMLElement>();
  let hovered: string | null = null;
  let groundY = 0;
  let contextFrame: THREE.Group | null = null;
  let pressed: { x: number; y: number; id: string | null } | null = null;
  let drag: { id: string; plane: THREE.Plane; offset: THREE.Vector3; start: THREE.Vector3; moved: boolean } | null = null;
  const raycaster = new THREE.Raycaster(), pointer = new THREE.Vector2();
  const changed = () => { needsRender = true; }; controls.addEventListener('change', changed);

  function resize() {
    if (host.clientWidth < 2 || host.clientHeight < 2) return;
    const width = Math.max(1, host.clientWidth), height = Math.max(1, host.clientHeight);
    renderer.setSize(width, height); labels.setSize(width, height); camera.aspect = width / height; camera.updateProjectionMatrix(); needsRender = true; if (state) overview();
  }
  const observer = new ResizeObserver(resize); observer.observe(host); resize();
  const shellObserver = new ResizeObserver(() => { if (state && !disposed) overview(); });
  for (const element of document.querySelectorAll('.place-header, .place-orientation, .governance-access, #build-bar, .view-pages, #inspector')) shellObserver.observe(element);
  function cast(event: PointerEvent) { const rect = canvas.getBoundingClientRect(); pointer.set((event.clientX - rect.left) / rect.width * 2 - 1, -(event.clientY - rect.top) / rect.height * 2 + 1); raycaster.setFromCamera(pointer, camera); }
  function pick(event: PointerEvent): string | null { cast(event); return raycaster.intersectObjects(picks, false)[0]?.object.userData.subjectId ?? null; }
  function activate(id: string | null) {
    if (id && state?.project.contexts.some(context => context.id === id)) { callbacks.enter?.(id); return; }
    callbacks.select(id);
  }
  function beginDrag(event: PointerEvent, id: string) {
    const group = groups.get(id); if (!group || !state?.project.concepts.some(concept => concept.id === id) || event.button !== 0) return;
    event.preventDefault(); event.stopPropagation(); cast(event);
    const plane = new THREE.Plane(new THREE.Vector3(0, 1, 0), -group.position.y), hit = raycaster.ray.intersectPlane(plane, new THREE.Vector3()); if (!hit) return;
    flight = null; controls.enabled = false; drag = { id, plane, offset: group.position.clone().sub(hit), start: group.position.clone(), moved: false }; host.classList.add('is-dragging');
  }
  function pointerDown(event: PointerEvent) {
    flight = null;
    if (event.button !== 0) return; const id = pick(event); pressed = { x: event.clientX, y: event.clientY, id };
    if (event.shiftKey && id) beginDrag(event, id);
  }
  function pointerMove(event: PointerEvent) {
    if (drag) { cast(event); const point = raycaster.ray.intersectPlane(drag.plane, new THREE.Vector3()); if (!point) return; point.add(drag.offset); groups.get(drag.id)?.position.copy(point); drag.moved = point.distanceTo(drag.start) > .03; needsRender = true; renderer.shadowMap.needsUpdate = true; return; }
    if (event.target !== canvas) { if (hovered) hoverLabels.get(hovered)?.classList.remove('is-hovered'); hovered = null; return; }
    const id = pick(event); if (id === hovered) return;
    if (hovered) hoverLabels.get(hovered)?.classList.remove('is-hovered'); hovered = id;
    if (id) hoverLabels.get(id)?.classList.add('is-hovered'); canvas.style.cursor = id ? 'pointer' : 'grab';
  }
  function pointerUp(event: PointerEvent) {
    if (drag) { const position = groups.get(drag.id)?.position; if (position && drag.moved) callbacks.move(drag.id, [round(position.x), state?.project.concepts.find(subject => subject.id === drag!.id)?.position[1] ?? 0, round(position.z)]); drag = null; controls.enabled = true; pressed = null; host.classList.remove('is-dragging'); return; }
    if (pressed && event.target === canvas && Math.hypot(event.clientX - pressed.x, event.clientY - pressed.y) < 5) activate(pressed.id);
    pressed = null;
  }
  function keyboard(event: KeyboardEvent) {
    if (event.key === 'Home') { event.preventDefault(); overview(); }
    if (event.key === '+' || event.key === '=') zoom(1);
    if (event.key === '-') zoom(-1);
    if (event.key === 'Escape') flight = null;
    if (event.key === 'Enter' && state?.selectedId) activate(state.selectedId);
  }
  canvas.addEventListener('pointerdown', pointerDown, true); canvas.addEventListener('keydown', keyboard);
  canvas.addEventListener('wheel', stopFlight, { passive: true }); window.addEventListener('pointermove', pointerMove); window.addEventListener('pointerup', pointerUp); window.addEventListener('pointercancel', pointerUp);
  function stopFlight() { flight = null; }

  function markPickables(group: THREE.Object3D, id: string) { group.traverse(object => { if (object instanceof THREE.Mesh) { object.userData.subjectId = id; picks.push(object); } }); }
  function label(subject: Subject, ghost = false) {
    const context = !('kind' in subject), button = document.createElement('button'); button.type = 'button';
    button.className = `study-label atlas-label atlas-label--${context ? 'context' : 'concept'}${ghost ? ' study-label--ghost' : ''}`;
    button.dataset.subjectId = subject.id; button.dataset.subjectKind = context ? 'context' : subject.kind; button.dataset.nodeState = ghost ? 'baseline' : 'current';
    button.setAttribute('aria-pressed', String(state?.selectedId === subject.id));
    const name = document.createElement('span'); name.className = 'study-name'; name.textContent = subject.name; button.append(name);
    const detail = document.createElement('small'); detail.textContent = context ? `${state!.project.concepts.filter(term => term.contextId === subject.id).length} recorded entries` : subject.kind === 'unclassified' ? 'Open question · kind unresolved' : subject.kind.replaceAll('_', ' '); button.append(detail);
    button.setAttribute('aria-label', `${subject.name}, ${context ? 'enter context' : detail.textContent}`);
    if (context) { button.dataset.recordedTerms = String(state!.project.concepts.filter(term => term.contextId === subject.id).length); button.title = 'Bounded context · Region area follows the full recorded term count; footprint follows saved layout.'; }
    if (view?.externalIds.includes(subject.id)) { const external = document.createElement('span'); external.className = 'study-external'; external.textContent = `↗ ${state?.project.contexts.find(context => context.id === ('contextId' in subject ? subject.contextId : subject.id))?.name ?? 'External context'}`; external.title = 'Recorded subject outside the current context'; button.append(external); }
    button.addEventListener('click', event => { event.stopPropagation(); if (!event.shiftKey) activate(subject.id); });
    button.addEventListener('pointerdown', event => { if (event.shiftKey) beginDrag(event, subject.id); });
    hoverLabels.set(subject.id, button);
    const object = new CSS2DObject(button); textObjects.push(object); return object;
  }
  function actor(subject: Subject, ghost = false) {
    const context = !('kind' in subject), group = new THREE.Group(); group.position.set(subject.position[0], groundY, subject.position[2]);
    const count = context ? state!.project.concepts.filter(term => term.contextId === subject.id).length : 0;
    const extent = regionExtent(count);
    const body = context ? contextForm(state!.project.concepts.filter(term => term.contextId === subject.id)) : conceptForm(subject.kind);
    group.add(body); markPickables(body, subject.id);
    if (ghost) body.traverse(object => { if (object instanceof THREE.Mesh) { const material = object.material as THREE.MeshStandardMaterial; material.color.setHex(ACCENT); material.wireframe = true; material.transparent = true; material.opacity = .55; object.castShadow = false; } });
    if (!context && !ghost) {
      const contracts = conceptContracts(state!.project, subject);
      // Contract marks belong only to the selected subject; the full text lives in its intentional review.
      if (view?.anchorId === subject.id && contracts.invariants.length) { const band = piece(new THREE.BoxGeometry(3.8, .09, .16), ACCENT); band.position.set(0, .5, 1.65); group.add(band); }
      if (view?.anchorId === subject.id && contracts.assertions.length) { const gate = piece(new THREE.BoxGeometry(.16, 1.5, .16), GRAPHITE); gate.position.set(2, .9, 0); group.add(gate); const lintel = piece(new THREE.BoxGeometry(.9, .16, .16), ACCENT); lintel.position.set(2.35, 1.6, 0); group.add(lintel); }
    }
    if (state?.selectedId === subject.id) {
      const ring = new THREE.Mesh(new THREE.RingGeometry(context ? 5.1 : 2.65, context ? 5.16 : 2.71, 64), new THREE.MeshBasicMaterial({ color: ACCENT, side: THREE.DoubleSide })); ring.rotation.x = -Math.PI / 2; ring.position.y = .03; group.add(ring);
    }
    const title = label(subject, ghost); title.position.set(0, .12, context ? extent.depth * .12 : 2.3); group.add(title);
    group.userData.domainActor = true; group.userData.subjectId = subject.id;
    groups.set(subject.id, group); locations.set(subject.id, group.position.clone()); world.add(group);
    if (state?.mode === 'baseline' && state.baseline && !ghost) {
      const old = [...state.baseline.project.contexts, ...state.baseline.project.concepts].find(item => item.id === subject.id);
      const meaningChanged = old && [...new Set([...Object.keys(old), ...Object.keys(subject)])].filter(key => key !== 'position').some(key => JSON.stringify((old as unknown as Record<string, unknown>)[key]) !== JSON.stringify((subject as unknown as Record<string, unknown>)[key]));
      const contractsChanged = old && 'kind' in old && 'kind' in subject && JSON.stringify(conceptContracts(state.baseline.project, old)) !== JSON.stringify(conceptContracts(state.project, subject));
      if (!old || meaningChanged || contractsChanged) {
        const width = context ? 5 : 2.35, depth = context ? 4 : 1.9;
        const outline = new THREE.LineLoop(new THREE.BufferGeometry().setFromPoints([new THREE.Vector3(-width, .07, -depth), new THREE.Vector3(width, .07, -depth), new THREE.Vector3(width, .07, depth), new THREE.Vector3(-width, .07, depth)]), new THREE.LineBasicMaterial({ color: ACCENT }));
        group.add(outline); title.element.dataset.baselineChange = old ? 'changed' : 'added';
        title.element.title = old ? 'Recorded meaning changed since model snapshot' : 'Added since model snapshot';
      }
      if (old && old.position.some((coordinate, index) => coordinate !== subject.position[index])) {
        const line = new THREE.Line(new THREE.BufferGeometry().setFromPoints([new THREE.Vector3(old.position[0], groundY + .2, old.position[2]), group.position.clone().add(new THREE.Vector3(0, .2, 0))]), new THREE.LineDashedMaterial({ color: ACCENT, dashSize: .35, gapSize: .25, transparent: true, opacity: .8 })); line.computeLineDistances(); world.add(line);
      }
    }
  }
  function focusedRoutes() {
    if (!state || view?.level !== 'detail') return;
    const visible = new Set(view.ids);
    const relations = state.project.relationships.filter(relation => visible.has(relation.source) && visible.has(relation.target) && (relation.id === state!.selectedId || relation.source === view!.anchorId || relation.target === view!.anchorId));
    const authored = relations.map(relation => ({ ...relation, derived: false }));
    for (const concept of state.project.concepts) {
      if (!visible.has(concept.id) || !concept.ownerId || !visible.has(concept.ownerId) || !['entity', 'repository', 'factory'].includes(concept.kind)) continue;
      const source = concept.kind === 'entity' ? concept.ownerId : concept.id, target = concept.kind === 'entity' ? concept.id : concept.ownerId;
      if (source !== view.anchorId && target !== view.anchorId) continue;
      if (authored.some(relation => (relation.source === source && relation.target === target) || (relation.source === target && relation.target === source))) continue;
      authored.push({ id: concept.id, source, target, label: concept.kind === 'entity' ? 'owns member' : concept.kind === 'repository' ? 'repository for' : 'creates', derived: true });
    }
    for (const relation of authored) {
      const from = locations.get(relation.source), to = locations.get(relation.target); if (!from || !to) continue;
      const semantics = relation.derived ? { direction: 'forward' as const, meaning: relation.label } : relationshipDescription(state.project, relation);
      const a = from.clone().add(new THREE.Vector3(0, .3, 0)), b = to.clone().add(new THREE.Vector3(0, .3, 0));
      const direction = b.clone().sub(a).normalize(); if (a.distanceTo(b) > 6) { a.addScaledVector(direction, 2.4); b.addScaledVector(direction, -2.4); }
      const middle = a.clone().lerp(b, .5); middle.y += Math.min(1.7, a.distanceTo(b) * .035);
      const curve = new THREE.QuadraticBezierCurve3(a, middle, b);
      const material = new THREE.LineDashedMaterial({ color: relation.id === state.selectedId ? ACCENT : GRAPHITE, transparent: true, opacity: .5, dashSize: semantics.direction === 'none' ? .25 : 1000, gapSize: .25 });
      const line = new THREE.Line(new THREE.BufferGeometry().setFromPoints(curve.getPoints(30)), material); line.computeLineDistances(); world.add(line);
      const arrow = (t: number, reverse: boolean) => { const mesh = piece(new THREE.ConeGeometry(.25, .65, 3), relation.id === state!.selectedId ? ACCENT : GRAPHITE); mesh.position.copy(curve.getPoint(t)); mesh.quaternion.setFromUnitVectors(new THREE.Vector3(0, 1, 0), curve.getTangent(t).normalize().multiplyScalar(reverse ? -1 : 1)); mesh.userData.subjectId = relation.id; world.add(mesh); picks.push(mesh); };
      if (semantics.direction !== 'none') arrow(.94, false); if (semantics.direction === 'both') arrow(.06, true);
      if (!relation.derived) locations.set(relation.id, middle);
    }
  }
  function membershipFrames() {
    contextFrame = null;
    if (!state || !view || view.level === 'world') return;
    const home = view.subjectContextId ?? view.contextId;
    const terms = state.project.concepts.filter(term => term.contextId === home);
    if (terms.length) {
      const bounds = new THREE.Box3().setFromPoints(terms.map(term => new THREE.Vector3(term.position[0], 0, term.position[2])));
      const center = bounds.getCenter(new THREE.Vector3());
      const frame = borderedPolygon(territoryVertices(terms).map(point => point.sub(new THREE.Vector2(center.x, center.z))), CONTEXT, .22, true);
      frame.position.set(center.x, -.08, center.z); frame.userData.currentContextFrame = home;
      world.add(frame); contextFrame = frame;
    } else {
      const context = state.project.contexts.find(context => context.id === home);
      if (context) { const frame = borderedPolygon(territoryVertices([]), CONTEXT, .22, true); frame.position.set(context.position[0], -.08, context.position[2]); frame.userData.currentContextFrame = home; world.add(frame); contextFrame = frame; }
    }
    for (const root of state.project.concepts.filter(term => term.kind === 'aggregate' && view!.ids.includes(term.id))) {
      const members = state.project.concepts.filter(term => term.ownerId === root.id && term.kind === 'entity');
      const shown = [root, ...members.filter(term => view!.ids.includes(term.id))];
      const bounds = new THREE.Box3().setFromPoints(shown.map(term => new THREE.Vector3(term.position[0], 0, term.position[2]))).expandByVector(new THREE.Vector3(3, 0, 3));
      const center = bounds.getCenter(new THREE.Vector3()), size = bounds.getSize(new THREE.Vector3());
      const ids = new Set(shown.map(term => term.id));
      const includesUnrelated = state.project.concepts.some(term => !ids.has(term.id) && bounds.containsPoint(new THREE.Vector3(term.position[0], 0, term.position[2])));
      const components = includesUnrelated ? shown.map(term => ({ center: new THREE.Vector3(term.position[0], 0, term.position[2]), width: 5.4, depth: 5 })) : [{ center, width: Math.max(5.4, size.x), depth: Math.max(5, size.z) }];
      for (const component of components) {
        const frame = borderedRegion(component.width, component.depth, GRAPHITE, .48, false); frame.position.copy(component.center); frame.userData.aggregateEnclosure = root.id; world.add(frame);
      }
      if (includesUnrelated) for (const member of shown.slice(1)) {
        const line = new THREE.Line(new THREE.BufferGeometry().setFromPoints([new THREE.Vector3(root.position[0], .18, root.position[2]), new THREE.Vector3(member.position[0], .18, member.position[2])]), new THREE.LineDashedMaterial({ color: GRAPHITE, dashSize: .28, gapSize: .18, transparent: true, opacity: .55 }));
        line.computeLineDistances(); line.userData.aggregateEnclosure = root.id; world.add(line);
      }
      const rootLabel = hoverLabels.get(root.id); if (rootLabel) { rootLabel.dataset.recordedMembers = String(members.length); rootLabel.title = `Aggregate consistency boundary · ${members.length} recorded member${members.length === 1 ? '' : 's'}`; }
    }
  }
  function update(next: SceneState) {
    if (disposed) return;
    const fallbackScope = next.scopeId ?? (next.aggregateId ? next.project.concepts.find(item => item.id === next.aggregateId)?.contextId : null);
    state = next; view = next.view ?? projectView(next.project, { scopeId: fallbackScope, selectedId: next.selectedId });
    // Defensive cap also covers malformed caller-provided views. No offstage geometry or proxy actors survives.
    view = { ...view, ids: [...new Set(view.ids)].slice(0, 8) };
    scene.remove(world); disposeObject(world); world = new THREE.Group(); scene.add(world); locations.clear(); groups.clear(); picks.length = 0; textObjects.length = 0; hoverLabels.clear(); hovered = null;
    const current = new Map<string, Subject>([...next.project.contexts, ...next.project.concepts].map(subject => [subject.id, subject]));
    groundY = 0; floor.position.y = -.12;
    for (const id of view.ids) { const subject = current.get(id); if (subject) actor(subject); else { const previous = next.baseline && [...next.baseline.project.contexts, ...next.baseline.project.concepts].find(item => item.id === id); if (previous) actor(previous, true); } }
    focusedRoutes(); membershipFrames();
    host.dataset.viewLevel = view.level; host.dataset.visibleSubjects = String(groups.size);
    needsRender = true; renderer.shadowMap.needsUpdate = true;
    const signature = `${view.level}|${view.contextId}|${view.anchorId ?? view.selectedId}|${view.page}`;
    if (signature !== priorView || initialFrame) { priorView = signature; overview(); initialFrame = false; }
  }
  function travel(position: THREE.Vector3, target: THREE.Vector3) {
    needsRender = true;
    if (reducedMotion) { camera.position.copy(position); controls.target.copy(target); controls.update(); flight = null; host.dataset.cameraState = 'settled'; }
    else { flight = { from: camera.position.clone(), fromTarget: controls.target.clone(), to: position, target, started: performance.now() }; host.dataset.cameraState = 'moving'; }
  }
  function overview(overhead = false) {
    const hostRect = host.getBoundingClientRect(), width = hostRect.width, height = hostRect.height;
    if (width < 2 || height < 2) return;
    const orientation = document.querySelector('.place-orientation')?.getBoundingClientRect();
    const topControls = [...document.querySelectorAll('.place-header, .governance-access')].map(element => element.getBoundingClientRect().bottom - hostRect.top);
    const bottomControls = [...document.querySelectorAll('#build-bar, .map-navigation, .view-pages')]
      .filter(element => (element as HTMLElement).offsetParent !== null).map(element => element.getBoundingClientRect().top - hostRect.top);
    const mobile = width <= 650;
    const safe = {
      left: mobile ? 16 : Math.min(width * .34, Math.max(32, (orientation?.right ?? 0) - hostRect.left + 24)),
      right: width - (mobile ? 16 : 32),
      top: mobile ? Math.max(110, (orientation?.bottom ?? 0) - hostRect.top + 18) : Math.max(100, ...topControls) + 16,
      bottom: mobile ? Math.min(height - 215, ...bottomControls) - 12 : Math.min(height - 120, ...bottomControls) - 12,
    };
    // The nonmodal editor can retain a useful map beside it on desktop; phone editing owns the surface.
    const editor = document.querySelector<HTMLElement>('#inspector');
    if (!mobile && editor && !editor.hidden && editor.offsetParent !== null) {
      const rightEdge = editor.getBoundingClientRect().left - hostRect.left - 18;
      if (rightEdge - safe.left > 220) safe.right = Math.min(safe.right, rightEdge);
    }
    safe.bottom = Math.max(safe.top + 120, safe.bottom);
    // Shift the optical center into the unobscured stage. Orbit still revolves around the real model center.
    camera.setViewOffset(width, height, width / 2 - (safe.left + safe.right) / 2, height / 2 - (safe.top + safe.bottom) / 2, width, height);
    const points: THREE.Vector3[] = [];
    const domainBox = new THREE.Box3();
    for (const group of [...groups.values(), ...(contextFrame && view?.level === 'context' ? [contextFrame] : [])]) {
      group.updateWorldMatrix(true, true); const bounds = new THREE.Box3().setFromObject(group);
      if (bounds.isEmpty()) continue;
      domainBox.union(bounds);
      // An irregular boundary has no corners at the corners of its bounding box.
      // Fit the rendered geometry itself so entering does not reserve imaginary empty territory.
      group.traverse(object => {
        if (!(object instanceof THREE.Mesh || object instanceof THREE.Line)) return;
        const vertices = object.geometry.getAttribute('position');
        if (!vertices) return;
        for (let index = 0; index < vertices.count; index++) points.push(new THREE.Vector3().fromBufferAttribute(vertices, index).applyMatrix4(object.matrixWorld));
      });
    }
    if (domainBox.isEmpty()) domainBox.set(new THREE.Vector3(-12, 0, -8), new THREE.Vector3(12, 4, 8));
    const allBounds = new THREE.Box3().setFromPoints(points.length ? points : [domainBox.min, domainBox.max]);
    const center = allBounds.getCenter(new THREE.Vector3());
    // Preserve heading across layers; change altitude so entering is a physical approach.
    const altitude = view?.level === 'detail' ? 1.05 : view?.level === 'context' ? 1.25 : 1.65;
    const direction = overhead ? new THREE.Vector3(0, 1, .001).normalize() : new THREE.Vector3(.38, altitude, 1).normalize();
    const probe = camera.clone();
    function fits(distance: number) {
      probe.position.copy(center).addScaledVector(direction, distance); probe.lookAt(center); probe.updateMatrixWorld(true);
      return points.every(point => { const p = point.clone().project(probe), x = (p.x + 1) * width / 2, y = (1 - p.y) * height / 2;
        return p.z > -1 && p.z < 1 && x >= safe.left + 34 && x <= safe.right - 34 && y >= safe.top + 10 && y <= safe.bottom - 34;
      });
    }
    let low = 12, high = Math.max(36, allBounds.getSize(new THREE.Vector3()).length() * 4);
    while (!fits(high) && high < 10000) high *= 1.5;
    for (let iteration = 0; iteration < 22; iteration++) { const middle = (low + high) / 2; if (fits(middle)) high = middle; else low = middle; }
    controls.maxDistance = Math.max(800, high * 2); camera.far = Math.max(1400, high * 4); camera.updateProjectionMatrix();
    travel(center.clone().addScaledVector(direction, high), center);
  }
  function focus(id: string) {
    const target = locations.get(id); if (!target) return;
    if (view?.level === 'detail' && view.ids.includes(id)) { overview(); return; }
    const distance = state?.project.contexts.some(item => item.id === id) ? 37 : 22;
    travel(target.clone().add(new THREE.Vector3(distance * .3, distance * .7, distance)), target.clone());
  }
  function topView() { overview(true); }
  function zoom(direction: number) { flight = null; const offset = camera.position.clone().sub(controls.target); offset.setLength(THREE.MathUtils.clamp(offset.length() * (direction > 0 ? .8 : 1.25), controls.minDistance, controls.maxDistance)); camera.position.copy(controls.target).add(offset); controls.update(); needsRender = true; }
  function screenPosition(id: string) { const position = locations.get(id); if (!position) return null; const p = position.clone().project(camera), rect = host.getBoundingClientRect(); if (p.z > 1) return null; return { x: rect.left + (p.x + 1) * rect.width / 2, y: rect.top + (1 - p.y) * rect.height / 2 }; }
  function keepLabelsSeparate() {
    const rect = host.getBoundingClientRect(), placed: DOMRect[] = [];
    const occupied = [...document.querySelectorAll<HTMLElement>('.place-header, .place-orientation, .governance-access, .focus-controls, #build-bar, .map-navigation, .view-pages, #keeper-action, #inspector, #onyx-support')]
      .filter(element => !element.hidden && element.offsetParent !== null).map(element => element.getBoundingClientRect());
    const project = (position: THREE.Vector3) => { const p = position.clone().project(camera); return { x: rect.left + (p.x + 1) * rect.width / 2, y: rect.top + (1 - p.y) * rect.height / 2 }; };
    const markers = [...groups].map(([id, group]) => ({ id, point: project(group.position.clone().add(new THREE.Vector3(0, .45, 0))) }));
    for (const object of textObjects) { object.element.style.marginTop = '0px'; object.element.style.marginLeft = '0px'; }
    const objects = textObjects.filter(object => object.element.classList.contains('atlas-label')).map(object => ({ object, bounds: object.element.getBoundingClientRect() }))
      .sort((a, b) => {
        const priority = (object: CSS2DObject) => object.element.dataset.subjectId === state?.selectedId ? 3 : object.element.dataset.subjectKind === 'aggregate' ? 2 : object.element.dataset.subjectKind === 'entity' ? 1 : 0;
        return priority(b.object) - priority(a.object) || a.bounds.top - b.bounds.top || a.bounds.left - b.bounds.left;
      });
    const intersects = (a: DOMRect, b: DOMRect, gap = 5) => a.left < b.right + gap && a.right > b.left - gap && a.top < b.bottom + gap && a.bottom > b.top - gap;
    const fragment = document.createDocumentFragment();
    for (const { object, bounds } of objects) {
      if (bounds.width === 0 || bounds.top > rect.bottom || bounds.bottom < rect.top) continue;
      const id = object.element.dataset.subjectId!, anchor = markers.find(marker => marker.id === id)?.point;
      const obstacles = [...occupied, ...placed, ...markers.filter(marker => marker.id !== id).map(marker => new DOMRect(marker.point.x - 10, marker.point.y - 10, 20, 20))];
      let best: { box: DOMRect; score: number } | null = null;
      // Search close to the original name first. Only the annotation moves; actor coordinates never do.
      for (const radius of [0, 18, 32, 48, 68, 92, 120, 154, 190]) {
        const candidates = radius === 0 ? [[0, 0]] : Array.from({ length: 16 }, (_, index) => { const angle = index * Math.PI / 8; return [Math.cos(angle) * radius, Math.sin(angle) * radius]; });
        for (const [dx, dy] of candidates) {
          const x = THREE.MathUtils.clamp(bounds.x + dx, rect.left + 10, Math.max(rect.left + 10, rect.right - bounds.width - 10));
          const y = THREE.MathUtils.clamp(bounds.y + dy, rect.top + 10, Math.max(rect.top + 10, rect.bottom - bounds.height - 10));
          const box = new DOMRect(x, y, bounds.width, bounds.height);
          const collisions = obstacles.reduce((count, obstacle) => count + Number(intersects(box, obstacle)), 0);
          let crossings = 0;
          if (anchor && Math.hypot(x - bounds.x, y - bounds.y) > 11) {
            const end = { x: THREE.MathUtils.clamp(anchor.x, box.left + 3, box.right - 3), y: THREE.MathUtils.clamp(anchor.y, box.top + 3, box.bottom - 3) };
            const vx = end.x - anchor.x, vy = end.y - anchor.y, lengthSquared = vx * vx + vy * vy;
            crossings = markers.filter(marker => marker.id !== id).reduce((count, marker) => {
              const t = lengthSquared ? THREE.MathUtils.clamp(((marker.point.x - anchor.x) * vx + (marker.point.y - anchor.y) * vy) / lengthSquared, 0, 1) : 0;
              return count + Number(Math.hypot(marker.point.x - anchor.x - t * vx, marker.point.y - anchor.y - t * vy) < 12);
            }, 0);
          }
          const score = collisions * 100_000_000 + crossings * 1_000_000 + (x - bounds.x) ** 2 + (y - bounds.y) ** 2;
          if (!best || score < best.score) best = { box, score };
        }
        if (best && best.score < 1_000_000) break;
      }
      if (!best) continue;
      const dx = best.box.x - bounds.x, dy = best.box.y - bounds.y;
      object.element.style.marginLeft = `${dx}px`; object.element.style.marginTop = `${dy}px`; placed.push(best.box);
      if (anchor && Math.hypot(dx, dy) > 11) {
        const endX = THREE.MathUtils.clamp(anchor.x, best.box.left + 3, best.box.right - 3), endY = THREE.MathUtils.clamp(anchor.y, best.box.top + 3, best.box.bottom - 3);
        const line = document.createElementNS('http://www.w3.org/2000/svg', 'line');
        line.dataset.labelFor = id; line.setAttribute('x1', String(anchor.x - rect.left)); line.setAttribute('y1', String(anchor.y - rect.top)); line.setAttribute('x2', String(endX - rect.left)); line.setAttribute('y2', String(endY - rect.top)); fragment.append(line);
      }
    }
    leaders.setAttribute('viewBox', `0 0 ${rect.width} ${rect.height}`); leaders.replaceChildren(fragment);
  }
  function animate() {
    if (disposed) return; frame = requestAnimationFrame(animate); const flying = !!flight;
    if (flight) { const t = Math.min(1, (performance.now() - flight.started) / 650), ease = t * t * (3 - 2 * t); camera.position.lerpVectors(flight.from, flight.to, ease); controls.target.lerpVectors(flight.fromTarget, flight.target, ease); if (t === 1) flight = null; }
    const moved = controls.update();
    host.dataset.cameraState = flying || moved ? 'moving' : 'settled'; host.dataset.cameraDistance = camera.position.distanceTo(controls.target).toFixed(2);
    if (needsRender || moved || flying) { renderer.render(scene, camera); labels.render(scene, camera); keepLabelsSeparate(); host.dataset.drawCalls = String(renderer.info.render.calls); needsRender = false; }
  }
  animate();
  return { update, focus, overview, topView, zoom, screenPosition, dispose() { disposed = true; cancelAnimationFrame(frame); observer.disconnect(); shellObserver.disconnect(); controls.removeEventListener('change', changed); controls.dispose(); canvas.removeEventListener('pointerdown', pointerDown, true); canvas.removeEventListener('keydown', keyboard); canvas.removeEventListener('wheel', stopFlight); window.removeEventListener('pointermove', pointerMove); window.removeEventListener('pointerup', pointerUp); window.removeEventListener('pointercancel', pointerUp); disposeObject(scene); renderer.dispose(); host.remove(); } };
}

function round(value: number) { return Math.round(value * 100) / 100; }
function piece(geometry: THREE.BufferGeometry, color = STONE) { const mesh = new THREE.Mesh(geometry, new THREE.MeshStandardMaterial({ color, roughness: .92, metalness: .02 })); mesh.castShadow = true; mesh.receiveShadow = true; return mesh; }
function add(group: THREE.Group, geometry: THREE.BufferGeometry, color: number, x: number, y: number, z: number) { const object = piece(geometry, color); object.position.set(x, y, z); group.add(object); return object; }
function regionExtent(count: number) {
  // Area grows with the complete recorded term count, never the current page of eight.
  const width = Math.sqrt(180 + count * 18); return { width, depth: width * .76 };
}
function roundedOutline(width: number, depth: number, radius = 1.4) {
  const x = width / 2, z = depth / 2, r = Math.min(radius, x / 3, z / 3), shape = new THREE.Shape();
  shape.moveTo(-x + r, -z); shape.lineTo(x - r, -z); shape.quadraticCurveTo(x, -z, x, -z + r);
  shape.lineTo(x, z - r); shape.quadraticCurveTo(x, z, x - r, z); shape.lineTo(-x + r, z); shape.quadraticCurveTo(-x, z, -x, z - r);
  shape.lineTo(-x, -z + r); shape.quadraticCurveTo(-x, -z, -x + r, -z); return shape;
}
function borderedRegion(width: number, depth: number, color: number, height: number, fill: boolean) {
  const group = new THREE.Group();
  if (fill) { const geometry = new THREE.ExtrudeGeometry(roundedOutline(width, depth), { depth: .13, bevelEnabled: false, curveSegments: 10 }); geometry.rotateX(-Math.PI / 2); add(group, geometry, 0xdce8e7, 0, -.02, 0); }
  const points = roundedOutline(width, depth).getPoints(64).map(point => new THREE.Vector3(point.x, height, point.y));
  const perimeter = new THREE.LineLoop(new THREE.BufferGeometry().setFromPoints(points), new THREE.LineBasicMaterial({ color })); group.add(perimeter);
  // A single open threshold communicates entry, not an architectural Layer.
  add(group, new THREE.BoxGeometry(width * .5 - .9, height, .16), color, -width * .25 - .45, height / 2, depth / 2);
  add(group, new THREE.BoxGeometry(width * .5 - .9, height, .16), color, width * .25 + .45, height / 2, depth / 2);
  add(group, new THREE.BoxGeometry(.16, height, depth - 1.6), color, -width / 2, height / 2, 0);
  add(group, new THREE.BoxGeometry(.16, height, depth - 1.6), color, width / 2, height / 2, 0);
  add(group, new THREE.BoxGeometry(width - 1.6, height, .16), color, 0, height / 2, -depth / 2);
  return group;
}
function territoryVertices(terms: Concept[]) {
  if (!terms.length) return [new THREE.Vector2(-6,-4), new THREE.Vector2(5,-5), new THREE.Vector2(6,3), new THREE.Vector2(-4,5)];
  const points = terms.flatMap(term => [[-4,-3], [3,-4], [4,3], [-3,4]].map(([x,z]) => new THREE.Vector2(term.position[0]+x, term.position[2]+z)));
  points.sort((a,b) => a.x-b.x || a.y-b.y);
  const cross = (a: THREE.Vector2,b: THREE.Vector2,c: THREE.Vector2) => (b.x-a.x)*(c.y-a.y)-(b.y-a.y)*(c.x-a.x);
  const lower: THREE.Vector2[] = [], upper: THREE.Vector2[] = [];
  for (const point of points) { while(lower.length>=2 && cross(lower[lower.length-2],lower[lower.length-1],point)<=0) lower.pop(); lower.push(point); }
  for (const point of [...points].reverse()) { while(upper.length>=2 && cross(upper[upper.length-2],upper[upper.length-1],point)<=0) upper.pop(); upper.push(point); }
  return [...lower.slice(0,-1),...upper.slice(0,-1)];
}
function borderedPolygon(points: THREE.Vector2[], color: number, height: number, fill: boolean) {
  const group = new THREE.Group(), shape = new THREE.Shape(points.map(point => new THREE.Vector2(point.x, -point.y)));
  if (fill) { const geometry = new THREE.ExtrudeGeometry(shape, {depth:.13,bevelEnabled:false}); geometry.rotateX(-Math.PI/2); add(group, geometry, 0xdce8e7, 0, -.02, 0); }
  const entrance = points.reduce((winner, point, index) => (point.y+points[(index+1)%points.length].y) > (points[winner].y+points[(winner+1)%points.length].y) ? index : winner,0);
  points.forEach((point,index)=>{
    const next=points[(index+1)%points.length],delta=next.clone().sub(point),length=delta.length(),middle=point.clone().lerp(next,.5),direction=delta.clone().normalize();
    const segment = (midpoint:THREE.Vector2,span:number) => {const wall=add(group,new THREE.BoxGeometry(span,height,.13),color,midpoint.x,height/2,midpoint.y);wall.rotation.y=-Math.atan2(delta.y,delta.x);};
    if(index===entrance && length>3){ segment(middle.clone().addScaledVector(direction,-length/4-.4),length/2-.8);segment(middle.clone().addScaledVector(direction,length/4+.4),length/2-.8); }
    else segment(middle,length);
  });
  return group;
}
function contextForm(terms: Concept[]) {
  const { width, depth } = regionExtent(terms.length), points = territoryVertices(terms), bounds = new THREE.Box2().setFromPoints(points), center=bounds.getCenter(new THREE.Vector2());
  const area=Math.abs(points.reduce((sum,p,index)=>{const q=points[(index+1)%points.length];return sum+p.x*q.y-q.x*p.y;},0))/2;
  const scale=Math.sqrt(width*depth/Math.max(area,1));
  const vertices=points.map(point=>point.clone().sub(center).multiplyScalar(scale)),group=borderedPolygon(vertices,CONTEXT,.35,true);
  const blocks=Math.min(terms.length,16),spacing=Math.min(.62,(width-4)/Math.max(1,blocks));
  for(let index=0;index<blocks;index++)add(group,new THREE.BoxGeometry(.28,.12,.5),CONTEXT,(index-(blocks-1)/2)*spacing,.13,-depth*.18);
  return group;
}
function conceptForm(kind: Concept['kind']) {
  const group = new THREE.Group(); add(group, new THREE.CylinderGeometry(1.7, 1.7, .12, 48), 0xd7e2e4, 0, .06, 0);
  if (kind === 'unclassified') {
    for (const x of [-1.25, 1.25]) for (const z of [-.9, .9]) add(group, new THREE.BoxGeometry(.4, .7, .4), STONE, x, .55, z);
    add(group, new THREE.BoxGeometry(2.9, .1, .1), 0x8f9b8b, 0, .3, -.9);
  } else if (kind === 'aggregate') {
    add(group, new THREE.CylinderGeometry(.8, .8, .5, 4), GRAPHITE, 0, .38, 0).rotation.y = Math.PI / 4;
    add(group, new THREE.BoxGeometry(.13, .7, .13), ACCENT, 0, .5, .9);
  } else if (kind === 'entity') {
    add(group, new THREE.BoxGeometry(1.9, 2.7, 1.6), STONE, -.2, 1.55, 0);
    add(group, new THREE.BoxGeometry(.17, 1.8, .12), GRAPHITE, .3, 1.5, .86);
  } else if (kind === 'value_object') {
    const gem = add(group, new THREE.OctahedronGeometry(1.45), 0xa5b9a8, 0, 1.85, 0); gem.rotation.y = .4;
  } else if (kind === 'domain_event') {
    add(group, new THREE.BoxGeometry(.17, 3.5, .17), GRAPHITE, -.55, 2, 0);
    add(group, new THREE.BoxGeometry(1.8, 1.7, .14), ACCENT, .2, 2.8, 0).rotation.y = -.2;
  } else if (kind === 'domain_service') {
    for (const x of [-1.1, 1.1]) add(group, new THREE.BoxGeometry(.5, 2.9, 1.3), STONE, x, 1.65, 0);
    add(group, new THREE.BoxGeometry(2.7, .5, 1.3), GRAPHITE, 0, 3, 0);
  } else if (kind === 'specification') {
    add(group, new THREE.BoxGeometry(2.2, 2.8, .5), GRAPHITE, 0, 1.65, 0);
    for (let index = 0; index < 3; index++) add(group, new THREE.BoxGeometry(1.45, .1, .06), CHALK, .1, 2.3 - index * .55, .28);
  } else if (kind === 'repository') {
    for (let index = 0; index < 3; index++) add(group, new THREE.BoxGeometry(.48, 2.7, 2.1), STONE, (index - 1) * .9, 1.55, 0);
    add(group, new THREE.BoxGeometry(3.3, .2, 2.6), GRAPHITE, 0, 2.95, 0);
  } else {
    add(group, new THREE.BoxGeometry(2.8, 1.5, 2), STONE, 0, 1, 0);
    add(group, new THREE.CylinderGeometry(.6, .9, 1.7, 4), GRAPHITE, -.6, 2.4, 0);
    add(group, new THREE.BoxGeometry(.6, 2.8, .6), ACCENT, 1.1, 1.7, -.5);
  }
  group.scale.y = .6; return group;
}
function disposeObject(object: THREE.Object3D) { const geometries = new Set<THREE.BufferGeometry>(), materials = new Set<THREE.Material>(); object.traverse(item => { if (item instanceof CSS2DObject) item.element.remove(); if (item instanceof THREE.Mesh || item instanceof THREE.Line) { if (item.geometry) geometries.add(item.geometry); for (const material of Array.isArray(item.material) ? item.material : [item.material]) materials.add(material); } }); geometries.forEach(geometry => geometry.dispose()); materials.forEach(material => { (material as THREE.MeshBasicMaterial).map?.dispose(); material.dispose(); }); }
function fallback(host: HTMLElement, callbacks: SceneCallbacks): SceneController {
  host.classList.add('study-fallback');
  function update(state: SceneState) {
    const view = state.view ?? projectView(state.project, { scopeId: state.scopeId, selectedId: state.selectedId }); host.replaceChildren(); host.dataset.viewLevel = view.level;
    const note = document.createElement('p'); note.textContent = '3D is unavailable. The same focused subjects remain available below.'; host.append(note);
    for (const id of view.ids.slice(0, 8)) { const subject = [...state.project.contexts, ...state.project.concepts].find(item => item.id === id); if (!subject) continue; const button = document.createElement('button'); button.className = 'atlas-label'; button.dataset.subjectId = id; button.textContent = subject.name; button.addEventListener('click', () => 'kind' in subject ? callbacks.select(id) : callbacks.enter?.(id)); host.append(button); }
  }
  return { update, focus(id) { Array.from(host.querySelectorAll<HTMLButtonElement>('[data-subject-id]')).find(item => item.dataset.subjectId === id)?.focus(); }, overview() { host.scrollTop = 0; }, topView() { host.scrollTop = 0; }, zoom() {}, dispose() { host.remove(); }, screenPosition() { return null; } };
}
