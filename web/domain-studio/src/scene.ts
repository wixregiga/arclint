import * as THREE from 'three';
import { OrbitControls } from 'three/addons/controls/OrbitControls.js';
import { CSS2DObject, CSS2DRenderer } from 'three/addons/renderers/CSS2DRenderer.js';
import type { Concept, DomainContext, DomainProject, SceneCallbacks, SceneController, SceneState } from './contracts';
import { conceptContracts, relationshipDescription } from './model-evidence';
import { projectView, keeperFor, type FocusView } from './view-state';
import './scene.css';

const CHALK = 0xecede6, GRAPHITE = 0x25302e, STONE = 0xc2c7bb, ACCENT = 0xb74937;
const UI_KEEPER = '__interface_keeper__';
type Subject = Concept | DomainContext;
type Flight = { from: THREE.Vector3; fromTarget: THREE.Vector3; to: THREE.Vector3; target: THREE.Vector3; started: number };

/** This stage displays only the subjects allocated by FocusView. It never constructs domain hierarchy. */
export function createScene(container: HTMLElement, callbacks: SceneCallbacks): SceneController {
  const host = document.createElement('div'); host.className = 'atlas-scene study-scene'; container.append(host);
  let renderer: THREE.WebGLRenderer;
  try { renderer = new THREE.WebGLRenderer({ antialias: true, alpha: false, powerPreference: 'high-performance' }); }
  catch { return fallback(host, callbacks); }
  renderer.setPixelRatio(Math.min(devicePixelRatio || 1, 2)); renderer.setClearColor(CHALK, 1); renderer.outputColorSpace = THREE.SRGBColorSpace;
  renderer.toneMapping = THREE.ACESFilmicToneMapping; renderer.toneMappingExposure = 1.05;
  renderer.shadowMap.enabled = true; renderer.shadowMap.type = THREE.PCFSoftShadowMap; renderer.shadowMap.autoUpdate = false; renderer.shadowMap.needsUpdate = true;
  const canvas = renderer.domElement; canvas.tabIndex = 0; canvas.setAttribute('aria-label', 'Architectural domain study. Drag to orbit, right-drag to pan, scroll to zoom. Select a context to enter. Shift-drag a concept to arrange its saved position.'); host.append(canvas);
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
  let keeperLocation = new THREE.Vector3(), hovered: string | null = null;
  let groundY = 0;
  let keeperBody: THREE.Group | null = null;
  let pressed: { x: number; y: number; id: string | null } | null = null;
  let drag: { id: string; plane: THREE.Plane; offset: THREE.Vector3; start: THREE.Vector3; moved: boolean } | null = null;
  const raycaster = new THREE.Raycaster(), pointer = new THREE.Vector2();
  const changed = () => { needsRender = true; }; controls.addEventListener('change', changed);

  function resize() {
    const width = Math.max(1, host.clientWidth), height = Math.max(1, host.clientHeight);
    renderer.setSize(width, height); labels.setSize(width, height); camera.aspect = width / height; camera.updateProjectionMatrix(); needsRender = true; if (state) overview();
  }
  const observer = new ResizeObserver(resize); observer.observe(host); resize();
  function cast(event: PointerEvent) { const rect = canvas.getBoundingClientRect(); pointer.set((event.clientX - rect.left) / rect.width * 2 - 1, -(event.clientY - rect.top) / rect.height * 2 + 1); raycaster.setFromCamera(pointer, camera); }
  function pick(event: PointerEvent): string | null { cast(event); return raycaster.intersectObjects(picks, false)[0]?.object.userData.subjectId ?? null; }
  function activate(id: string | null) {
    if (id === UI_KEEPER) { callbacks.keeper?.(); return; }
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
    if (event.target !== canvas) return;
    const id = pick(event); if (id === hovered) return;
    if (hovered) hoverLabels.get(hovered)?.classList.remove('is-hovered'); hovered = id;
    if (id) hoverLabels.get(id)?.classList.add('is-hovered'); canvas.style.cursor = id ? 'pointer' : 'grab';
  }
  function pointerUp(event: PointerEvent) {
    if (drag) { const position = groups.get(drag.id)?.position; if (position && drag.moved) callbacks.move(drag.id, [round(position.x), round(position.y), round(position.z)]); drag = null; controls.enabled = true; pressed = null; host.classList.remove('is-dragging'); return; }
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
    const detail = document.createElement('small'); detail.textContent = context ? 'Enter context ↗' : subject.kind === 'unclassified' ? 'Unclassified · draft' : subject.kind.replaceAll('_', ' '); button.append(detail);
    button.setAttribute('aria-label', `${subject.name}, ${context ? 'enter context' : detail.textContent}`);
    if (view?.externalIds.includes(subject.id)) { const external = document.createElement('span'); external.className = 'study-external'; external.textContent = '↗'; external.title = 'Recorded subject outside the current context'; button.append(external); }
    button.addEventListener('click', event => { event.stopPropagation(); if (!event.shiftKey) activate(subject.id); });
    button.addEventListener('pointerdown', event => { if (event.shiftKey) beginDrag(event, subject.id); });
    hoverLabels.set(subject.id, button);
    const object = new CSS2DObject(button); textObjects.push(object); return object;
  }
  function actor(subject: Subject, ghost = false) {
    const context = !('kind' in subject), group = new THREE.Group(); group.position.set(...subject.position);
    const body = context ? contextForm(subject.id) : conceptForm(subject.kind);
    const elevation = Math.max(0, subject.position[1] - groundY);
    if (elevation > .04) add(body, new THREE.BoxGeometry(context ? 9 : 3.8, elevation, context ? 7 : 3), 0xd1d6ca, 0, -elevation / 2, 0);
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
    const title = label(subject, ghost); title.position.set(0, .1, context ? 5.5 : 2.8); group.add(title);
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
        title.element.title = old ? 'Recorded meaning changed since baseline' : 'Added since baseline';
      }
      if (old && old.position.some((coordinate, index) => coordinate !== subject.position[index])) {
        const line = new THREE.Line(new THREE.BufferGeometry().setFromPoints([new THREE.Vector3(...old.position).add(new THREE.Vector3(0, .2, 0)), group.position.clone().add(new THREE.Vector3(0, .2, 0))]), new THREE.LineDashedMaterial({ color: ACCENT, dashSize: .35, gapSize: .25, transparent: true, opacity: .8 })); line.computeLineDistances(); world.add(line);
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
  function visibleBounds() {
    const box = new THREE.Box3();
    for (const id of view?.ids ?? []) { const point = locations.get(id); if (!point) continue; const context = state?.project.contexts.some(item => item.id === id); const radius = context ? 6 : 3.5; box.expandByPoint(point.clone().add(new THREE.Vector3(radius, context ? 7 : 4, radius + 2))); box.expandByPoint(point.clone().sub(new THREE.Vector3(radius, .2, radius))); }
    if (box.isEmpty()) box.set(new THREE.Vector3(-14, 0, -10), new THREE.Vector3(14, 5, 10));
    return { box, center: box.getCenter(new THREE.Vector3()), size: box.getSize(new THREE.Vector3()) };
  }
  function groundAndKeeper() {
    const { center, size } = visibleBounds();
    if (view?.level === 'context') {
      const ground = piece(new THREE.BoxGeometry(Math.max(18, size.x + 5), .1, Math.max(16, size.z + 4)), 0xe2e5da); ground.position.set(center.x, groundY - .055, center.z); ground.castShadow = false; world.add(ground);
    }
    // The keeper is an interface guide at a stable entry threshold, never a persisted domain subject.
    const forward = new THREE.Vector3(.38, 0, 1).normalize(), right = new THREE.Vector3(1, 0, -.38).normalize();
    keeperLocation = center.clone().setY(groundY).addScaledVector(forward, size.z * .29 + 2).addScaledVector(right, size.x * .38 + 4);
    const keeper = keeperForm(view?.level ?? 'world'); keeperBody = keeper; keeper.position.copy(keeperLocation); keeper.rotation.y = -.12; keeper.userData.interfaceGuide = true;
    const guide = keeperFor(view!, state?.lens ?? 'meaning');
    const button = document.createElement('button'); button.className = 'study-keeper-name'; button.type = 'button'; button.dataset.keeper = guide.name; button.textContent = guide.name; button.setAttribute('aria-label', `${guide.name}, ${guide.role}. ${guide.action}`); button.title = guide.action; button.addEventListener('click', () => callbacks.keeper?.());
    const name = new CSS2DObject(button); name.position.set(0, .1, 2.2); keeper.add(name); hoverLabels.set(UI_KEEPER, button); textObjects.push(name);
    markPickables(keeper, UI_KEEPER); world.add(keeper);
    const threshold = piece(new THREE.BoxGeometry(5.5, .04, 4.3), 0xd7dcd0); threshold.position.copy(keeperLocation).setY(groundY - .02); threshold.castShadow = false; world.add(threshold);
  }
  function update(next: SceneState) {
    if (disposed) return;
    const fallbackScope = next.scopeId ?? (next.aggregateId ? next.project.concepts.find(item => item.id === next.aggregateId)?.contextId : null);
    state = next; view = next.view ?? projectView(next.project, { scopeId: fallbackScope, selectedId: next.selectedId });
    // Defensive cap also covers malformed caller-provided views. No offstage geometry or proxy actors survives.
    view = { ...view, ids: [...new Set(view.ids)].slice(0, 8) };
    scene.remove(world); disposeObject(world); world = new THREE.Group(); scene.add(world); locations.clear(); groups.clear(); picks.length = 0; textObjects.length = 0; hoverLabels.clear(); hovered = null;
    const current = new Map<string, Subject>([...next.project.contexts, ...next.project.concepts].map(subject => [subject.id, subject]));
    const heights = view.ids.flatMap(id => current.has(id) ? [current.get(id)!.position[1]] : []); groundY = heights.length ? Math.min(...heights) : 0; floor.position.y = groundY - .03;
    for (const id of view.ids) { const subject = current.get(id); if (subject) actor(subject); else { const previous = next.baseline && [...next.baseline.project.contexts, ...next.baseline.project.concepts].find(item => item.id === id); if (previous) actor(previous, true); } }
    focusedRoutes(); groundAndKeeper();
    host.dataset.viewLevel = view.level; host.dataset.visibleSubjects = String(groups.size); host.dataset.keeper = keeperFor(view, next.lens ?? 'meaning').name;
    needsRender = true; renderer.shadowMap.needsUpdate = true;
    const signature = `${view.level}|${view.contextId}|${view.anchorId ?? view.selectedId}|${view.page}`;
    if (signature !== priorView || initialFrame) { priorView = signature; overview(); initialFrame = false; }
  }
  function travel(position: THREE.Vector3, target: THREE.Vector3) {
    needsRender = true;
    if (reducedMotion) { camera.position.copy(position); controls.target.copy(target); controls.update(); flight = null; host.dataset.cameraState = 'settled'; }
    else { flight = { from: camera.position.clone(), fromTarget: controls.target.clone(), to: position, target, started: performance.now() }; host.dataset.cameraState = 'moving'; }
  }
  function overview() {
    const hostRect = host.getBoundingClientRect(), width = hostRect.width, height = hostRect.height;
    const orientation = document.querySelector('.place-orientation')?.getBoundingClientRect();
    const header = document.querySelector('.place-header')?.getBoundingClientRect();
    const footer = [...document.querySelectorAll('.focus-controls, .view-pages, #keeper-action')]
      .filter(element => (element as HTMLElement).offsetParent !== null).map(element => element.getBoundingClientRect().top - hostRect.top);
    const safe = {
      left: Math.min(width * .34, Math.max(32, (orientation?.right ?? 0) - hostRect.left + 24)),
      right: width - 32,
      top: Math.max(88, (header?.bottom ?? 0) - hostRect.top + 24),
      bottom: Math.max(height * .65, Math.min(height - 54, ...footer) - 26),
    };
    // Shift the optical center into the unobscured stage. Orbit still revolves around the real model center.
    camera.setViewOffset(width, height, width / 2 - (safe.left + safe.right) / 2, height / 2 - (safe.top + safe.bottom) / 2, width, height);
    const points: THREE.Vector3[] = [];
    const domainBox = new THREE.Box3();
    for (const group of [...groups.values(), ...(keeperBody ? [keeperBody] : [])]) {
      group.updateWorldMatrix(true, true); const bounds = new THREE.Box3().setFromObject(group);
      if (bounds.isEmpty()) continue;
      if (group !== keeperBody) domainBox.union(bounds);
      for (const x of [bounds.min.x, bounds.max.x]) for (const y of [bounds.min.y, bounds.max.y]) for (const z of [bounds.min.z, bounds.max.z]) points.push(new THREE.Vector3(x, y, z));
    }
    if (domainBox.isEmpty()) domainBox.set(new THREE.Vector3(-12, 0, -8), new THREE.Vector3(12, 4, 8));
    const allBounds = new THREE.Box3().setFromPoints(points.length ? points : [domainBox.min, domainBox.max]);
    const center = allBounds.getCenter(new THREE.Vector3());
    // Preserve heading across layers; change altitude so entering is a physical approach.
    const altitude = view?.level === 'detail' ? .65 : view?.level === 'context' ? .82 : 1.02;
    const direction = new THREE.Vector3(.38, altitude, 1).normalize();
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
  function topView() { const { center, size } = visibleBounds(); const span = Math.max(size.x / camera.aspect, size.z, 22); travel(center.clone().add(new THREE.Vector3(0, span * 1.7, .01)), center); }
  function zoom(direction: number) { flight = null; const offset = camera.position.clone().sub(controls.target); offset.setLength(THREE.MathUtils.clamp(offset.length() * (direction > 0 ? .8 : 1.25), controls.minDistance, controls.maxDistance)); camera.position.copy(controls.target).add(offset); controls.update(); needsRender = true; }
  function screenPosition(id: string) { const position = locations.get(id); if (!position) return null; const p = position.clone().project(camera), rect = host.getBoundingClientRect(); if (p.z > 1) return null; return { x: rect.left + (p.x + 1) * rect.width / 2, y: rect.top + (1 - p.y) * rect.height / 2 }; }
  function keepLabelsSeparate() {
    const rect = host.getBoundingClientRect(), placed: DOMRect[] = [];
    for (const object of textObjects) { object.element.style.marginTop = '0px'; object.element.style.marginLeft = '0px'; }
    const objects = textObjects.filter(object => object.element.classList.contains('atlas-label')).sort((a, b) => a.getWorldPosition(new THREE.Vector3()).z - b.getWorldPosition(new THREE.Vector3()).z);
    for (const object of objects) {
      const element = object.element, bounds = element.getBoundingClientRect(); if (bounds.width === 0 || bounds.top > rect.bottom || bounds.bottom < rect.top) continue;
      let offset = 0;
      while (placed.some(other => bounds.left < other.right + 5 && bounds.right > other.left - 5 && bounds.top + offset < other.bottom + 5 && bounds.bottom + offset > other.top - 5) && offset < 72) offset += 18;
      element.style.marginTop = `${offset}px`; placed.push(new DOMRect(bounds.x, bounds.y + offset, bounds.width, bounds.height));
    }
  }
  function animate() {
    if (disposed) return; frame = requestAnimationFrame(animate); const flying = !!flight;
    if (flight) { const t = Math.min(1, (performance.now() - flight.started) / 650), ease = t * t * (3 - 2 * t); camera.position.lerpVectors(flight.from, flight.to, ease); controls.target.lerpVectors(flight.fromTarget, flight.target, ease); if (t === 1) flight = null; }
    const moved = controls.update();
    host.dataset.cameraState = flying || moved ? 'moving' : 'settled'; host.dataset.cameraDistance = camera.position.distanceTo(controls.target).toFixed(2);
    if (needsRender || moved || flying) { renderer.render(scene, camera); labels.render(scene, camera); keepLabelsSeparate(); host.dataset.drawCalls = String(renderer.info.render.calls); needsRender = false; }
  }
  animate();
  return { update, focus, overview, topView, zoom, screenPosition, dispose() { disposed = true; cancelAnimationFrame(frame); observer.disconnect(); controls.removeEventListener('change', changed); controls.dispose(); canvas.removeEventListener('pointerdown', pointerDown, true); canvas.removeEventListener('keydown', keyboard); canvas.removeEventListener('wheel', stopFlight); window.removeEventListener('pointermove', pointerMove); window.removeEventListener('pointerup', pointerUp); window.removeEventListener('pointercancel', pointerUp); disposeObject(scene); renderer.dispose(); host.remove(); } };
}

function hash(value: string) { let n = 0; for (const character of value) n = Math.imul(n, 31) + character.charCodeAt(0) | 0; return Math.abs(n); }
function round(value: number) { return Math.round(value * 100) / 100; }
function piece(geometry: THREE.BufferGeometry, color = STONE) { const mesh = new THREE.Mesh(geometry, new THREE.MeshStandardMaterial({ color, roughness: .92, metalness: .02 })); mesh.castShadow = true; mesh.receiveShadow = true; return mesh; }
function add(group: THREE.Group, geometry: THREE.BufferGeometry, color: number, x: number, y: number, z: number) { const object = piece(geometry, color); object.position.set(x, y, z); group.add(object); return object; }
function contextForm(id: string) {
  const group = new THREE.Group(), form = hash(id) % 8;
  add(group, new THREE.BoxGeometry(9, .3, 7), 0xd6dacd, 0, .15, 0);
  if (form === 0) {
    for (const x of [-2.6, 2.6]) add(group, new THREE.BoxGeometry(1.5, 6, 5), STONE, x, 3.25, 0);
    add(group, new THREE.BoxGeometry(6.8, 1.3, 5), STONE, 0, 5.7, 0);
    add(group, new THREE.BoxGeometry(.12, 4.2, .14), ACCENT, -1.8, 2.5, 2.57);
  } else if (form === 1) {
    add(group, new THREE.BoxGeometry(3.6, 5.8, 3.2), STONE, -1.4, 3.2, -.4);
    add(group, new THREE.BoxGeometry(6.4, 1.25, 4.8), STONE, .7, 5.1, 0);
    add(group, new THREE.BoxGeometry(5.2, 1.1, 3.8), 0xadb6a8, 1.3, 2.9, .3);
    add(group, new THREE.BoxGeometry(4, .09, .13), ACCENT, .8, 4.5, 2.45);
  } else if (form === 2) {
    const shape = new THREE.Shape(); shape.absarc(0, 0, 3.5, 0, Math.PI * 2, false); const hole = new THREE.Path(); hole.absarc(0, 0, 1.85, 0, Math.PI * 2, true); shape.holes.push(hole);
    const geometry = new THREE.ExtrudeGeometry(shape, { depth: 5.2, bevelEnabled: false, curveSegments: 40 }); geometry.rotateX(-Math.PI / 2); add(group, geometry, STONE, 0, .3, 0);
    add(group, new THREE.BoxGeometry(.12, 4.1, .14), ACCENT, 0, 2.5, 3.55);
  } else if (form === 3) {
    add(group, new THREE.BoxGeometry(2, 6, 5.4), STONE, -2, 3.3, 0).rotation.z = -.12;
    add(group, new THREE.BoxGeometry(2, 6, 5.4), STONE, 2, 3.3, 0).rotation.z = .12;
    add(group, new THREE.BoxGeometry(2.5, .7, 4.8), 0xb0b9ab, 0, 2.8, 0);
    add(group, new THREE.BoxGeometry(.1, 3, .15), ACCENT, 1, 4.7, 2.75);
  } else if (form === 4) {
    add(group, new THREE.BoxGeometry(6.8, 2, 5.5), STONE, 0, 1.3, 0);
    add(group, new THREE.BoxGeometry(5.4, 2, 4.3), STONE, -.65, 3.25, -.45);
    add(group, new THREE.BoxGeometry(3.9, 1.6, 3.1), STONE, -1.1, 5.05, -.8);
    add(group, new THREE.BoxGeometry(3.5, .08, .14), ACCENT, -.8, 4.3, 1.72);
  } else if (form === 5) {
    for (const x of [-2.5, 2.5]) for (const z of [-1.8, 1.8]) add(group, new THREE.BoxGeometry(1.15, 5.2, 1.15), STONE, x, 2.9, z);
    add(group, new THREE.BoxGeometry(7.2, 1.05, 5.7), STONE, 0, 5.55, 0);
    add(group, new THREE.BoxGeometry(.13, 3.6, .13), ACCENT, -1.86, 2.2, 2.42);
  } else if (form === 6) {
    for (let i = 0; i < 5; i++) add(group, new THREE.BoxGeometry(.9, 5.5, 4.6), STONE, (i - 2) * 1.35, 3.05, 0);
    add(group, new THREE.BoxGeometry(7, .65, 5.2), STONE, 0, 5.9, 0);
    add(group, new THREE.BoxGeometry(.13, 3.8, .13), ACCENT, -1.86, 2.4, 2.4);
  } else {
    add(group, new THREE.CylinderGeometry(1.9, 1.9, 5.8, 48), STONE, -1.9, 3.2, -1);
    add(group, new THREE.CylinderGeometry(1.9, 1.9, 5.8, 48), STONE, 1.9, 3.2, 1);
    add(group, new THREE.BoxGeometry(5.7, .65, 1.5), STONE, 0, 4.3, 0).rotation.y = -.45;
    add(group, new THREE.BoxGeometry(.12, 3.7, .14), ACCENT, 1.9, 2.4, 2.95);
  }
  return group;
}
function conceptForm(kind: Concept['kind']) {
  const group = new THREE.Group(); add(group, new THREE.BoxGeometry(3.8, .22, 3), 0xd1d7c9, 0, .11, 0);
  if (kind === 'unclassified') {
    for (const x of [-1.25, 1.25]) for (const z of [-.9, .9]) add(group, new THREE.BoxGeometry(.4, .7, .4), STONE, x, .55, z);
    add(group, new THREE.BoxGeometry(2.9, .1, .1), 0x8f9b8b, 0, .3, -.9);
  } else if (kind === 'aggregate') {
    add(group, new THREE.BoxGeometry(2.9, 1.4, 2.2), STONE, 0, .95, 0);
    add(group, new THREE.BoxGeometry(2, 1.5, 1.5), GRAPHITE, 0, 2.3, 0);
    add(group, new THREE.BoxGeometry(3.35, .25, 2.6), STONE, 0, 3.2, 0);
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
  return group;
}
function keeperForm(level: FocusView['level']) {
  const group = new THREE.Group();
  if (level === 'world') {
    const body = add(group, new THREE.CylinderGeometry(.65, 1, 1.6, 5), GRAPHITE, 0, 1.1, 0); body.rotation.y = Math.PI / 5;
    const head = add(group, new THREE.IcosahedronGeometry(.9, 0), 0x34403a, 0, 2.4, .07); head.scale.set(1, .94, .82);
    for (const x of [-.56, .56]) { const ear = add(group, new THREE.ConeGeometry(.35, .9, 3), GRAPHITE, x, 3.1, 0); ear.rotation.z = x < 0 ? .22 : -.22; }
    const muzzle = add(group, new THREE.ConeGeometry(.42, .6, 3), 0x68756a, 0, 2.15, .7); muzzle.rotation.x = Math.PI / 2;
    for (const x of [-.33, .33]) add(group, new THREE.SphereGeometry(.075, 8, 6), CHALK, x, 2.48, .76);
    add(group, new THREE.BoxGeometry(.16, .85, .16), ACCENT, -.4, 1.7, .65).rotation.z = -.18;
    for (const x of [-.5, .5]) add(group, new THREE.BoxGeometry(.45, .35, .8), GRAPHITE, x, .17, .25);
    group.scale.setScalar(1.25);
  } else {
    const inspector = level === 'detail';
    add(group, new THREE.CylinderGeometry(.65, .95, 2, 6), inspector ? GRAPHITE : 0x849385, 0, 1.5, 0);
    for (const x of [-.35, .35]) add(group, new THREE.BoxGeometry(.3, .6, .5), GRAPHITE, x, .35, .1);
    add(group, new THREE.SphereGeometry(.46, 12, 8), 0xb3bcae, 0, 2.9, 0);
    const hood = add(group, new THREE.SphereGeometry(.55, 12, 8, 0, Math.PI * 2, 0, Math.PI * .65), inspector ? GRAPHITE : 0x63766a, 0, 3, -.08); hood.rotation.x = -.24;
    for (const x of [-.7, .7]) { const arm = add(group, new THREE.CylinderGeometry(.15, .17, 1.4, 6), inspector ? GRAPHITE : 0x849385, x, 1.85, .05); arm.rotation.z = x < 0 ? -.16 : .32; }
    if (inspector) { const lens = add(group, new THREE.TorusGeometry(.31, .07, 6, 12), ACCENT, .88, 2, .55); lens.rotation.y = -.2; add(group, new THREE.BoxGeometry(.13, .75, .1), ACCENT, .82, 1.45, .52); }
    else { const ledger = add(group, new THREE.BoxGeometry(.78, 1.1, .22), 0xd2d6c7, -.67, 1.6, .6); ledger.rotation.z = -.12; add(group, new THREE.BoxGeometry(.13, 1.1, .24), ACCENT, -1.02, 1.6, .61); }
    group.scale.setScalar(1.15);
  }
  return group;
}
function disposeObject(object: THREE.Object3D) { const geometries = new Set<THREE.BufferGeometry>(), materials = new Set<THREE.Material>(); object.traverse(item => { if (item instanceof CSS2DObject) item.element.remove(); if (item instanceof THREE.Mesh || item instanceof THREE.Line) { if (item.geometry) geometries.add(item.geometry); for (const material of Array.isArray(item.material) ? item.material : [item.material]) materials.add(material); } }); geometries.forEach(geometry => geometry.dispose()); materials.forEach(material => { (material as THREE.MeshBasicMaterial).map?.dispose(); material.dispose(); }); }
function fallback(host: HTMLElement, callbacks: SceneCallbacks): SceneController {
  host.classList.add('study-fallback');
  function update(state: SceneState) {
    const view = state.view ?? projectView(state.project, { scopeId: state.scopeId, selectedId: state.selectedId }); host.replaceChildren(); host.dataset.viewLevel = view.level;
    const note = document.createElement('p'); note.textContent = '3D is unavailable. The same focused subjects remain available below.'; host.append(note);
    for (const id of view.ids.slice(0, 8)) { const subject = [...state.project.contexts, ...state.project.concepts].find(item => item.id === id); if (!subject) continue; const button = document.createElement('button'); button.className = 'atlas-label'; button.dataset.subjectId = id; button.textContent = subject.name; button.addEventListener('click', () => 'kind' in subject ? callbacks.select(id) : callbacks.enter?.(id)); host.append(button); }
    const guide = keeperFor(view, state.lens ?? 'meaning'), keeper = document.createElement('button'); keeper.textContent = guide.name; keeper.setAttribute('aria-label', `${guide.name}: ${guide.action}`); keeper.addEventListener('click', () => callbacks.keeper?.()); host.append(keeper);
  }
  return { update, focus(id) { Array.from(host.querySelectorAll<HTMLButtonElement>('[data-subject-id]')).find(item => item.dataset.subjectId === id)?.focus(); }, overview() { host.scrollTop = 0; }, topView() { host.scrollTop = 0; }, zoom() {}, dispose() { host.remove(); }, screenPosition() { return null; } };
}
