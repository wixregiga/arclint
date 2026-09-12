import * as THREE from 'three';
import { OrbitControls } from 'three/addons/controls/OrbitControls.js';
import { CSS2DObject, CSS2DRenderer } from 'three/addons/renderers/CSS2DRenderer.js';
import type { Concept, DomainContext, SceneCallbacks, SceneController, SceneState } from './contracts';
import './scene.css';

const INK = 0x273a38;
const RED = 0xa94332;
const TERRAIN = [0xa9b79d, 0xb7b59a, 0xa5b4b0, 0xb8b19b, 0xa9b3a0];
type Territory = { context: DomainContext; center: THREE.Vector3; radius: number; offset: THREE.Vector3; index: number };

/** The atlas is a view of recorded membership; visual placement never changes ownership. */
export function createScene(container: HTMLElement, callbacks: SceneCallbacks): SceneController {
  const host = document.createElement('div'); host.className = 'atlas-scene'; container.append(host);
  let renderer: THREE.WebGLRenderer;
  try { renderer = new THREE.WebGLRenderer({ antialias: true, alpha: false, powerPreference: 'high-performance' }); }
  catch { return fallbackScene(host, callbacks); }
  renderer.setPixelRatio(Math.min(devicePixelRatio || 1, 2));
  renderer.setClearColor(0xe5dfce, 1);
  renderer.outputColorSpace = THREE.SRGBColorSpace;
  renderer.toneMapping = THREE.ACESFilmicToneMapping;
  renderer.toneMappingExposure = .92;
  renderer.shadowMap.enabled = true;
  renderer.shadowMap.type = THREE.PCFSoftShadowMap;
  renderer.shadowMap.autoUpdate = false;
  renderer.shadowMap.needsUpdate = true;
  renderer.domElement.tabIndex = 0;
  renderer.domElement.setAttribute('aria-label', 'Domain atlas. Drag to orbit, right-drag to pan, scroll to zoom. Shift-drag a concept to arrange it. Double-click a context or aggregate to enter it.');
  host.append(renderer.domElement);
  const labels = new CSS2DRenderer(); labels.domElement.className = 'atlas-labels'; host.append(labels.domElement);
  const scene = new THREE.Scene(); scene.background = new THREE.Color(0xe5dfce); scene.fog = new THREE.Fog(0xe5dfce, 170, 390);
  const camera = new THREE.PerspectiveCamera(40, 1, .1, 1200); camera.position.set(55, 70, 90);
  const controls = new OrbitControls(camera, renderer.domElement);
  controls.listenToKeyEvents(renderer.domElement);
  const reducedMotion = matchMedia('(prefers-reduced-motion: reduce)').matches;
  controls.enableDamping = !reducedMotion; controls.dampingFactor = .1; controls.minDistance = 10; controls.maxDistance = 700; controls.maxPolarAngle = Math.PI * .46;
  scene.add(new THREE.HemisphereLight(0xf1efdf, 0x677d72, 1.55));
  const sunlight = new THREE.DirectionalLight(0xfff5e3, 2.25); sunlight.position.set(-45, 90, 35); sunlight.castShadow = true;
  sunlight.shadow.mapSize.set(2048, 2048); sunlight.shadow.camera.left = -140; sunlight.shadow.camera.right = 140; sunlight.shadow.camera.top = 140; sunlight.shadow.camera.bottom = -140; sunlight.shadow.camera.far = 260; sunlight.shadow.normalBias = .1;
  scene.add(sunlight);
  const backdrop = makeWater(); scene.add(backdrop);
  let world = new THREE.Group(); scene.add(world);
  let state: SceneState | null = null;
  let disposed = false;
  let needsRender = true;
  const markForRender = () => { needsRender = true; };
  controls.addEventListener('change', markForRender);
  let frame = 0;
  let hasFramed = false;
  let priorScope = '';
  let flight: { position: THREE.Vector3; target: THREE.Vector3; fromPosition: THREE.Vector3; fromTarget: THREE.Vector3; started: number } | null = null;
  const locations = new Map<string, THREE.Vector3>();
  const groups = new Map<string, THREE.Group>();
  const offsets = new Map<string, THREE.Vector3>();
  let territories: Territory[] = [];
  const picks: THREE.Object3D[] = [];
  const hoverLabels = new Map<string, HTMLElement>();
  const raycaster = new THREE.Raycaster();
  const pointer = new THREE.Vector2();
  let pressed: { x: number; y: number } | null = null;
  let lastClick: { id: string; time: number } | null = null;
  let selectTimer: ReturnType<typeof setTimeout> | null = null;
  let drag: { id: string; plane: THREE.Plane; offset: THREE.Vector3; original: THREE.Vector3; moved: boolean } | null = null;
  let hovered: string | null = null;

  function resize() {
    const width = Math.max(host.clientWidth, 1), height = Math.max(host.clientHeight, 1);
    renderer.setSize(width, height); labels.setSize(width, height); camera.aspect = width / height; camera.updateProjectionMatrix(); needsRender = true;
  }
  const observer = new ResizeObserver(resize); observer.observe(host); resize();
  function screenRay(event: PointerEvent) {
    const rect = renderer.domElement.getBoundingClientRect();
    pointer.set((event.clientX - rect.left) / rect.width * 2 - 1, -(event.clientY - rect.top) / rect.height * 2 + 1); raycaster.setFromCamera(pointer, camera);
  }
  function pick(event: PointerEvent): string | null { screenRay(event); return raycaster.intersectObjects(picks, false)[0]?.object.userData.subjectId ?? null; }
  function enterable(id: string) { return state?.project.contexts.some(context => context.id === id) || state?.project.concepts.some(concept => concept.id === id && concept.kind === 'aggregate'); }
  function activate(id: string | null) {
    const now = performance.now();
    if (selectTimer) { clearTimeout(selectTimer); selectTimer = null; }
    if (id && lastClick?.id === id && now - lastClick.time < 420 && enterable(id)) { lastClick = null; callbacks.enter?.(id); return; }
    lastClick = id ? { id, time: now } : null;
    // Keep an enterable label alive for the second click before selection rebuilds the view.
    if (id && enterable(id)) selectTimer = setTimeout(() => { selectTimer = null; callbacks.select(id); }, 240);
    else callbacks.select(id);
  }
  function beginDrag(event: PointerEvent, id: string) {
    const origin = groups.get(id)?.position;
    if (!origin || !state?.project.concepts.some(concept => concept.id === id) || event.button !== 0) return;
    event.preventDefault(); event.stopPropagation(); screenRay(event);
    const plane = new THREE.Plane(new THREE.Vector3(0, 1, 0), -origin.y);
    const hit = raycaster.ray.intersectPlane(plane, new THREE.Vector3()); if (!hit) return;
    drag = { id, plane, offset: origin.clone().sub(hit), original: origin.clone(), moved: false };
    controls.enabled = false; flight = null; host.classList.add('is-dragging');
  }
  function onDown(event: PointerEvent) {
    if (event.button !== 0) return; pressed = { x: event.clientX, y: event.clientY }; flight = null;
    if (event.shiftKey) { const id = pick(event); if (id) beginDrag(event, id); }
  }
  function onMove(event: PointerEvent) {
    if (drag) {
      screenRay(event); const hit = raycaster.ray.intersectPlane(drag.plane, new THREE.Vector3()); if (!hit) return;
      hit.add(drag.offset); groups.get(drag.id)?.position.copy(hit); drag.moved = hit.distanceTo(drag.original) > .05; needsRender = true; renderer.shadowMap.needsUpdate = true; return;
    }
    if (event.target !== renderer.domElement) return;
    const id = pick(event);
    if (id !== hovered) {
      if (hovered) hoverLabels.get(hovered)?.classList.remove('is-hovered');
      hovered = id; if (id) hoverLabels.get(id)?.classList.add('is-hovered');
      renderer.domElement.style.cursor = id ? 'pointer' : 'grab';
    }
  }
  function onUp(event: PointerEvent) {
    if (drag) {
      const position = groups.get(drag.id)?.position.clone();
      if (position && drag.moved) { position.sub(offsets.get(drag.id) ?? new THREE.Vector3()); position.y -= 1.6; callbacks.move(drag.id, [round(position.x), round(position.y), round(position.z)]); }
      drag = null; controls.enabled = true; host.classList.remove('is-dragging'); pressed = null; return;
    }
    if (pressed && Math.hypot(event.clientX - pressed.x, event.clientY - pressed.y) < 5 && event.target === renderer.domElement) activate(pick(event));
    pressed = null;
  }
  function onKey(event: KeyboardEvent) {
    if (event.key === 'Escape') callbacks.select(null);
    if (event.key === 'Enter' && state?.selectedId && enterable(state.selectedId)) { event.preventDefault(); callbacks.enter?.(state.selectedId); }
    if (event.key === 'Home') { event.preventDefault(); overview(); }
    if (event.key === '+' || event.key === '=') zoom(1);
    if (event.key === '-') zoom(-1);
  }
  renderer.domElement.addEventListener('pointerdown', onDown, true); renderer.domElement.addEventListener('keydown', onKey);
  window.addEventListener('pointermove', onMove); window.addEventListener('pointerup', onUp); window.addEventListener('pointercancel', onUp);

  function label(id: string, name: string, type: 'context' | 'concept' | 'route', ghost = false) {
    const button = document.createElement('button'); button.type = 'button'; button.className = `atlas-label atlas-label--${type}${ghost ? ' atlas-label--ghost' : ''}`;
    button.dataset.subjectId = id; button.setAttribute('aria-label', `${ghost ? 'Previous: ' : ''}${name}`); button.setAttribute('aria-pressed', String(state?.selectedId === id)); button.title = name;
    const text = document.createElement('span'); text.className = 'atlas-label-name'; text.textContent = name; button.append(text);
    if (type !== 'route' && state?.findings.some(finding => finding.subjectId === id)) {
      const badge = document.createElement('span'); badge.className = 'atlas-finding'; badge.textContent = '·'; badge.title = 'Model questions to review'; badge.setAttribute('aria-label', 'has model questions'); button.append(badge);
    }
    if (type === 'context') { const hint = document.createElement('small'); hint.textContent = 'BOUNDED CONTEXT'; button.append(hint); }
    if (state?.search && !name.toLowerCase().includes(state.search.toLowerCase())) button.classList.add('is-dim');
    if (state?.scopeId || state?.aggregateId) button.classList.add('is-scoped');
    if (ghost) { button.disabled = true; button.tabIndex = -1; }
    else {
      button.addEventListener('click', event => { event.stopPropagation(); if (!event.shiftKey) activate(id); });
      button.addEventListener('dblclick', event => { if (enterable(id)) { event.preventDefault(); event.stopPropagation(); if (selectTimer) { clearTimeout(selectTimer); selectTimer = null; } lastClick = null; callbacks.enter?.(id); } });
      button.addEventListener('pointerdown', event => { if (event.shiftKey) beginDrag(event, id); });
      button.addEventListener('keydown', event => { if (event.key === 'Enter' && enterable(id)) { event.preventDefault(); callbacks.enter?.(id); } });
      hoverLabels.set(id, button);
    }
    return new CSS2DObject(button);
  }

  function makeTerritory(territory: Territory, ghost = false) {
    const { context, center, radius, index } = territory;
    const group = new THREE.Group(); group.position.copy(center); group.position.y = 0;
    const shape = islandShape(radius, hash(context.id));
    if (ghost) {
      const edge = outline(shape, .16, RED, .65); group.add(edge);
    } else {
      const lower = slab(shape, 2.2, 0x8d9788); lower.position.y = -1.7; group.add(lower);
      const middle = slab(shape, .9, 0xb8b69f); middle.scale.set(.965, 1, .965); middle.position.y = .45; group.add(middle);
      const upper = slab(shape, .3, TERRAIN[index % TERRAIN.length]); upper.scale.set(.91, 1, .91); upper.position.y = 1.35; group.add(upper);
      [lower, middle, upper].forEach(mesh => { mesh.userData.subjectId = context.id; picks.push(mesh); });
      const edge = outline(shape, 1.66, 0x718675, .4); edge.scale.set(.91, 1, .91); group.add(edge);
      const shoreline = outline(shape, -1.5, 0xf4ead7, .75); shoreline.scale.set(1.025, 1, 1.025); group.add(shoreline);
      const shore2 = outline(shape, -1.53, 0xb2c2b5, .4); shore2.scale.set(1.08, 1, 1.08); group.add(shore2);
      // A few quiet stones and brush-like trees are terrain decoration, never selectable concepts.
      for (let i = 0; i < 7; i++) {
        const angle = i * 2.399 + index, distance = radius * (.68 + (i % 3) * .065);
        const x = Math.cos(angle) * distance, z = Math.sin(angle) * distance;
        if (i % 3 === 0) {
          const stone = mesh(new THREE.DodecahedronGeometry(.55 + i * .025, 0), 0x8a9384); stone.scale.set(1.6, .6, 1); stone.position.set(x, 1.6, z); stone.rotation.y = i; group.add(stone);
        } else group.add(tree(x, z, .75 + (i % 2) * .25));
      }
    }
    const title = label(context.id, context.name, 'context', ghost); title.position.set(0, 2.1, radius * .95 + 2.7); group.add(title);
    world.add(group); if (!ghost || !locations.has(context.id)) locations.set(context.id, center.clone().setY(1.6));
  }

  function makeConcept(concept: Concept, offset: THREE.Vector3, ghost = false) {
    const group = new THREE.Group(); group.position.set(...concept.position).add(offset); group.position.y += 1.6;
    const selected = state?.selectedId === concept.id;
    const building = architecture(concept.kind, ghost); group.add(building);
    const dim = !!state?.search && !concept.name.toLowerCase().includes(state.search.toLowerCase());
    building.traverse(object => {
      if (object instanceof THREE.Mesh) { object.userData.subjectId = concept.id; if (!ghost) picks.push(object); if (dim) { const material = object.material as THREE.MeshStandardMaterial; material.transparent = true; material.opacity = .24; } }
    });
    if (selected && !ghost) {
      const marker = new THREE.Mesh(new THREE.RingGeometry(2.75, 2.86, 64), new THREE.MeshBasicMaterial({ color: RED, side: THREE.DoubleSide, transparent: true, opacity: .8 })); marker.rotation.x = -Math.PI / 2; marker.position.y = .06; group.add(marker);
      const banner = mesh(new THREE.BoxGeometry(.04, 4.8, .04), 0x5d5441); banner.position.set(-2, 2.4, -.4); group.add(banner);
      const flag = mesh(new THREE.BoxGeometry(.9, 1.3, .025), RED); flag.position.set(-1.55, 3.9, -.4); group.add(flag);
    }
    const title = label(concept.id, ghost ? `${concept.name} · previous` : concept.name, 'concept', ghost); title.position.set(0, .4, 2.5); group.add(title);
    world.add(group);
    if (!ghost) { groups.set(concept.id, group); offsets.set(concept.id, offset); locations.set(concept.id, group.position.clone()); }
    else if (!locations.has(concept.id)) locations.set(concept.id, group.position.clone());
  }

  function route(source: string, target: string, id: string, text: string, derived = false, ghost = false, endpoints?: [THREE.Vector3, THREE.Vector3]) {
    const from = endpoints?.[0] ?? locations.get(source), to = endpoints?.[1] ?? locations.get(target); if (!from || !to) return;
    const a = from.clone().add(new THREE.Vector3(0, .4, 0)), b = to.clone().add(new THREE.Vector3(0, .4, 0));
    const middle = a.clone().lerp(b, .5); middle.y += Math.min(3.6, a.distanceTo(b) * .08);
    const curve = new THREE.QuadraticBezierCurve3(a, middle, b);
    const selected = state?.selectedId === id || state?.selectedId === source || state?.selectedId === target;
    if (ghost || derived) {
      const line = new THREE.Line(new THREE.BufferGeometry().setFromPoints(curve.getPoints(36)), new THREE.LineDashedMaterial({ color: ghost ? RED : 0x695f4b, dashSize: .55, gapSize: .25, transparent: true, opacity: ghost ? .6 : .45 })); line.computeLineDistances(); world.add(line);
    } else {
      const count = Math.min(80, Math.max(8, Math.round(a.distanceTo(b) * 1.25)));
      const planks = new THREE.InstancedMesh(new THREE.BoxGeometry(.75, .14, .32), new THREE.MeshStandardMaterial({ color: selected ? 0xa38a50 : 0x958874, roughness: 1 }), count);
      const temp = new THREE.Object3D();
      for (let index = 0; index < count; index++) { const t = .1 + index / count * .8; temp.position.copy(curve.getPoint(t)); temp.rotation.y = Math.atan2(curve.getTangent(t).x, curve.getTangent(t).z); temp.updateMatrix(); planks.setMatrixAt(index, temp.matrix); }
      planks.userData.subjectId = id; world.add(planks); picks.push(planks);
      const line = new THREE.Line(new THREE.BufferGeometry().setFromPoints(curve.getPoints(48)), new THREE.LineBasicMaterial({ color: selected ? RED : 0x716c57, transparent: true, opacity: .32 })); world.add(line);
    }
    const arrow = mesh(new THREE.ConeGeometry(.23, .65, 3), selected ? RED : 0x675e49); arrow.position.copy(curve.getPoint(.77)); arrow.quaternion.setFromUnitVectors(new THREE.Vector3(0, 1, 0), curve.getTangent(.77).normalize()); world.add(arrow); arrow.userData.subjectId = id; if (!ghost) picks.push(arrow);
    if (!ghost) { const title = label(id, text, 'route'); title.position.copy(curve.getPoint(.5)); title.element.classList.toggle('is-revealed', selected); world.add(title); }
    if (!derived && !locations.has(id)) locations.set(id, middle);
  }

  function update(next: SceneState) {
    if (disposed) return;
    needsRender = true; renderer.shadowMap.needsUpdate = true;
    state = next; scene.remove(world); disposeObject(world); world = new THREE.Group(); scene.add(world);
    locations.clear(); groups.clear(); offsets.clear(); picks.length = 0; hoverLabels.clear(); hovered = null;
    territories = layoutTerritories(next);
    const scope = next.scopeId ?? (next.aggregateId ? next.project.concepts.find(concept => concept.id === next.aggregateId)?.contextId : undefined);
    const visibleTerritories = scope ? territories.filter(item => item.context.id === scope) : territories;
    for (const territory of visibleTerritories) makeTerritory(territory);
    const visibleConcepts = next.project.concepts.filter(concept => (!scope || concept.contextId === scope) && (!next.aggregateId || concept.id === next.aggregateId || concept.ownerId === next.aggregateId));
    for (const concept of visibleConcepts) makeConcept(concept, territories.find(item => item.context.id === concept.contextId)?.offset ?? new THREE.Vector3());
    for (const relationship of next.project.relationships) route(relationship.source, relationship.target, relationship.id, relationship.label);
    for (const concept of visibleConcepts) {
      if (!concept.ownerId || !['entity', 'repository', 'factory'].includes(concept.kind)) continue;
      if (!next.project.concepts.some(owner => owner.id === concept.ownerId && owner.kind === 'aggregate')) continue;
      const source = concept.kind === 'entity' ? concept.ownerId : concept.id, target = concept.kind === 'entity' ? concept.id : concept.ownerId;
      if (!next.project.relationships.some(item => (item.source === source && item.target === target) || (item.source === target && item.target === source))) route(source, target, concept.id, concept.kind === 'entity' ? 'owns member' : concept.kind === 'repository' ? 'repository for' : 'creates', true);
    }
    // A low wall records actual aggregate membership, rather than inventing ownership from proximity.
    for (const aggregate of visibleConcepts.filter(concept => concept.kind === 'aggregate')) {
      const members = visibleConcepts.filter(concept => concept.ownerId === aggregate.id && concept.kind === 'entity'); if (!members.length) continue;
      const points = [aggregate, ...members].map(item => locations.get(item.id)!).filter(Boolean);
      const box = new THREE.Box3().setFromPoints(points); box.expandByScalar(3.2);
      const corners = [new THREE.Vector3(box.min.x, 1.9, box.min.z), new THREE.Vector3(box.max.x, 1.9, box.min.z), new THREE.Vector3(box.max.x, 1.9, box.max.z), new THREE.Vector3(box.min.x, 1.9, box.max.z)];
      for (let index = 0; index < 4; index++) { const from = corners[index], to = corners[(index + 1) % 4]; const wall = mesh(new THREE.BoxGeometry(from.distanceTo(to), .55, .25), 0x9b9987); wall.position.copy(from).lerp(to, .5); wall.rotation.y = -Math.atan2(to.z - from.z, to.x - from.x); world.add(wall); }
    }
    if (next.mode === 'baseline' && next.baseline) {
      const oldTerritories = layoutTerritories({ ...next, project: next.baseline.project });
      for (const context of next.baseline.project.contexts) {
        if (scope && context.id !== scope) continue;
        const current = next.project.contexts.find(item => item.id === context.id);
        if (!current || JSON.stringify(current) !== JSON.stringify(context)) { const old = oldTerritories.find(item => item.context.id === context.id); if (old) makeTerritory(old, true); }
      }
      for (const concept of next.baseline.project.concepts) {
        if (scope && concept.contextId !== scope) continue;
        if (next.aggregateId && concept.id !== next.aggregateId && concept.ownerId !== next.aggregateId) continue;
        const current = next.project.concepts.find(item => item.id === concept.id);
        if (!current || JSON.stringify(current) !== JSON.stringify(concept)) { const offset = (oldTerritories.find(item => item.context.id === concept.contextId)?.offset ?? new THREE.Vector3()).clone(); if (current && current.position.every((value, index) => value === concept.position[index])) offset.y += 3; makeConcept(concept, offset, true); }
      }
      for (const old of next.baseline.project.relationships) {
        if (next.project.relationships.some(item => item.id === old.id && JSON.stringify(item) === JSON.stringify(old))) continue;
        const previousPoint = (id: string) => {
          const concept = next.baseline!.project.concepts.find(item => item.id === id);
          const territory = oldTerritories.find(item => item.context.id === (concept?.contextId ?? id));
          if (scope && territory?.context.id !== scope) return null;
          if (concept) return new THREE.Vector3(...concept.position).add(territory?.offset ?? new THREE.Vector3()).add(new THREE.Vector3(0, 1.6, 0));
          return territory?.center.clone().setY(1.6) ?? null;
        };
        const from = previousPoint(old.source), to = previousPoint(old.target);
        if (from && to) route(old.source, old.target, old.id, old.label, false, true, [from, to]);
      }
    }
    host.classList.toggle('is-scoped', !!scope); host.classList.toggle('has-selection', !!next.selectedId);
    const scopeKey = `${scope ?? ''}/${next.aggregateId ?? ''}`;
    if (!hasFramed || scopeKey !== priorScope) { hasFramed = true; priorScope = scopeKey; overview(); }
  }

  function fitBounds() {
    const visible = territories.filter(item => locations.has(item.context.id));
    const box = new THREE.Box3();
    if (!state?.aggregateId) for (const territory of visible) { box.expandByPoint(territory.center.clone().add(new THREE.Vector3(territory.radius + 3, 5, territory.radius + 6))); box.expandByPoint(territory.center.clone().sub(new THREE.Vector3(territory.radius + 3, 0, territory.radius + 3))); }
    else groups.forEach(group => { box.expandByPoint(group.position.clone().addScalar(4)); box.expandByPoint(group.position.clone().subScalar(4)); });
    if (box.isEmpty()) return { center: new THREE.Vector3(), extent: new THREE.Vector3(45, 5, 45) };
    return { center: box.getCenter(new THREE.Vector3()), extent: box.getSize(new THREE.Vector3()) };
  }
  function travel(position: THREE.Vector3, target: THREE.Vector3) {
    needsRender = true;
    if (reducedMotion) { camera.position.copy(position); controls.target.copy(target); controls.update(); flight = null; }
    else flight = { position, target, fromPosition: camera.position.clone(), fromTarget: controls.target.clone(), started: performance.now() };
  }
  function overview() {
    const { center, extent } = fitBounds();
    const width = Math.max(extent.x, 25), depth = Math.max(extent.z, 25);
    const size = Math.max(width / Math.max(camera.aspect, .55), depth * .72);
    const distance = size * (state?.scopeId || state?.aggregateId ? 1.27 : 1.36);
    travel(center.clone().add(new THREE.Vector3(distance * .38, distance * .85, distance)), center);
  }
  function focus(id: string) {
    const point = locations.get(id); if (!point) return;
    const territory = territories.find(item => item.context.id === id);
    const distance = territory ? territory.radius * 3.9 : 24;
    const direction = camera.position.clone().sub(controls.target).normalize(); travel(point.clone().addScaledVector(direction, distance), point.clone());
  }
  function topView() { const { center, extent } = fitBounds(); const size = Math.max(extent.x / camera.aspect, extent.z); travel(center.clone().add(new THREE.Vector3(0, size * 1.65, .01)), center); }
  function zoom(direction: number) { flight = null; needsRender = true; const offset = camera.position.clone().sub(controls.target); offset.setLength(THREE.MathUtils.clamp(offset.length() * (direction > 0 ? .8 : 1.25), controls.minDistance, controls.maxDistance)); camera.position.copy(controls.target).add(offset); controls.update(); }
  function screenPosition(id: string) {
    const location = locations.get(id); if (!location) return null;
    const point = location.clone().project(camera), rect = host.getBoundingClientRect();
    if (point.z > 1) return null; return { x: rect.left + (point.x + 1) / 2 * rect.width, y: rect.top + (1 - point.y) / 2 * rect.height };
  }
  function animate() {
    if (disposed) return; frame = requestAnimationFrame(animate);
    const flying = !!flight;
    if (flight) { const progress = Math.min(1, (performance.now() - flight.started) / 650); const eased = progress * progress * (3 - 2 * progress); camera.position.lerpVectors(flight.fromPosition, flight.position, eased); controls.target.lerpVectors(flight.fromTarget, flight.target, eased); if (progress === 1) flight = null; }
    const moved = controls.update();
    host.classList.toggle('is-near', camera.position.distanceTo(controls.target) < 65);
    if (needsRender || moved || flying) {
      renderer.render(scene, camera); labels.render(scene, camera); needsRender = false;
      if (host.dataset.drawCalls !== String(renderer.info.render.calls)) host.dataset.drawCalls = String(renderer.info.render.calls);
    }
  }
  animate();
  return { update, focus, overview, topView, zoom, screenPosition, dispose() { disposed = true; if (selectTimer) clearTimeout(selectTimer); cancelAnimationFrame(frame); observer.disconnect(); controls.removeEventListener('change', markForRender); controls.dispose(); renderer.domElement.removeEventListener('pointerdown', onDown, true); renderer.domElement.removeEventListener('keydown', onKey); window.removeEventListener('pointermove', onMove); window.removeEventListener('pointerup', onUp); window.removeEventListener('pointercancel', onUp); disposeObject(scene); renderer.dispose(); host.remove(); } };
}

function layoutTerritories(state: SceneState): Territory[] {
  const territories = state.project.contexts.map((context, index) => {
    const members = state.project.concepts.filter(concept => concept.contextId === context.id);
    const radius = Math.max(10, ...members.map(concept => Math.hypot(concept.position[0] - context.position[0], concept.position[2] - context.position[2]) + 5));
    return { context, center: new THREE.Vector3(...context.position).setY(0), radius, offset: new THREE.Vector3(), index };
  });
  for (let pass = 0; pass < 24; pass++) for (let i = 0; i < territories.length; i++) for (let j = i + 1; j < territories.length; j++) {
    const a = territories[i], b = territories[j], delta = b.center.clone().sub(a.center); const distance = delta.length(), required = a.radius + b.radius + 4;
    if (distance >= required) continue;
    if (distance < .001) delta.set(Math.cos(j * 2.399), 0, Math.sin(j * 2.399)); else delta.divideScalar(distance);
    delta.multiplyScalar((required - distance) * .51); a.center.sub(delta); b.center.add(delta);
  }
  territories.forEach(territory => territory.offset.copy(territory.center).sub(new THREE.Vector3(...territory.context.position)).setY(0));
  return territories;
}
function hash(text: string) { let value = 0; for (const character of text) value = (value * 31 + character.charCodeAt(0)) | 0; return Math.abs(value); }
function round(value: number) { return Math.round(value * 100) / 100; }
function mesh(geometry: THREE.BufferGeometry, color: number) { const object = new THREE.Mesh(geometry, new THREE.MeshStandardMaterial({ color, roughness: .95, metalness: 0 })); object.castShadow = true; object.receiveShadow = true; return object; }
function islandShape(radius: number, seed: number) {
  const points: THREE.Vector2[] = [];
  for (let index = 0; index < 14; index++) { const angle = index / 14 * Math.PI * 2; const r = radius * (.91 + Math.sin(seed + index * 2.7) * .045 + Math.cos(index * 1.8) * .04); points.push(new THREE.Vector2(Math.cos(angle) * r, Math.sin(angle) * r)); }
  return new THREE.Shape(points);
}
function slab(shape: THREE.Shape, height: number, color: number) { const geometry = new THREE.ExtrudeGeometry(shape, { depth: height, bevelEnabled: true, bevelSize: .13, bevelThickness: .12, bevelSegments: 1, steps: 1 }); geometry.rotateX(-Math.PI / 2); return mesh(geometry, color); }
function outline(shape: THREE.Shape, y: number, color: number, opacity: number) { const points = shape.getPoints().map(point => new THREE.Vector3(point.x, y, -point.y)); return new THREE.LineLoop(new THREE.BufferGeometry().setFromPoints(points), new THREE.LineBasicMaterial({ color, transparent: true, opacity })); }
function tree(x: number, z: number, size: number) { const group = new THREE.Group(); group.position.set(x, 1.6, z); const trunk = mesh(new THREE.CylinderGeometry(.08, .1, 1.5, 5), 0x6b6652); trunk.position.y = .75; group.add(trunk); const crown = mesh(new THREE.DodecahedronGeometry(.9, 0), 0x708977); crown.scale.set(1.1, .55, .85); crown.position.set(.2, 1.7, 0); group.add(crown); group.scale.setScalar(size); return group; }
function roof(width: number, depth: number, color: number) {
  const shape = new THREE.Shape(); shape.moveTo(-width / 2, 0); shape.lineTo(-width * .12, .9); shape.lineTo(width * .12, .9); shape.lineTo(width / 2, 0); shape.closePath();
  const geometry = new THREE.ExtrudeGeometry(shape, { depth, bevelEnabled: false }); geometry.translate(0, 0, -depth / 2); return mesh(geometry, color);
}
function architecture(kind: Concept['kind'], ghost: boolean) {
  const group = new THREE.Group();
  const base = mesh(new THREE.BoxGeometry(3.5, .35, 2.8), 0xc9c4ae); base.position.y = .18; group.add(base);
  const roofColor = kind === 'aggregate' ? 0x455c71 : 0x4c5a53;
  if (kind === 'value_object' || kind === 'domain_event' || kind === 'specification') {
    const stone = mesh(new THREE.DodecahedronGeometry(1.2, 0), kind === 'domain_event' ? 0xaf7965 : 0x969f92); stone.scale.set(kind === 'specification' ? .8 : 1, 1.4, .85); stone.position.y = 1.7; group.add(stone);
    const cap = mesh(new THREE.BoxGeometry(1.45, .14, 1.1), 0xddd4bc); cap.position.y = 2.95; group.add(cap);
  } else {
    const tiers = kind === 'aggregate' ? 3 : kind === 'repository' ? 2 : 1;
    for (let tier = 0; tier < tiers; tier++) {
      const scale = 1 - tier * .16, y = .35 + tier * 1.75;
      const body = mesh(new THREE.BoxGeometry(2.2 * scale, 1.4, 1.7 * scale), kind === 'unclassified' ? 0xc8bfa4 : 0xd2c8ac); body.position.y = y + .7; group.add(body);
      const top = roof(3.5 * scale, 2.6 * scale, roofColor); top.position.y = y + 1.4; group.add(top);
      for (const x of [-.8, .8]) { const post = mesh(new THREE.BoxGeometry(.11, 1.4, .11), 0x74634d); post.position.set(x * scale, y + .7, .86 * scale); group.add(post); }
      const door = mesh(new THREE.BoxGeometry(.5 * scale, .85, .025), 0x6c7263); door.position.set(0, y + .45, .86 * scale); group.add(door);
    }
    if (kind === 'factory') { const chimney = mesh(new THREE.BoxGeometry(.55, 2.4, .55), 0x9c8c74); chimney.position.set(1.1, 1.55, -.55); group.add(chimney); }
    if (kind === 'domain_service') { const gate = roof(4.1, .45, 0xa94332); gate.position.set(0, 2.5, 1.6); group.add(gate); }
  }
  if (ghost) group.traverse(object => { if (object instanceof THREE.Mesh) { const material = object.material as THREE.MeshStandardMaterial; material.color.setHex(RED); material.wireframe = true; material.transparent = true; material.opacity = .38; object.castShadow = false; } });
  return group;
}
function makeWater() {
  const group = new THREE.Group();
  const water = mesh(new THREE.PlaneGeometry(1000, 1000), 0xc1ccba); water.rotation.x = -Math.PI / 2; water.position.y = -1.7; water.castShadow = false; group.add(water);
  const lines: number[] = [];
  for (let z = -160; z < 170; z += 4.5) for (let x = -180; x < 180; x += 4) { const wave = Math.sin(x * .045 + z * .08) * .7; lines.push(x, -1.67, z + wave, x + 2.3, -1.67, z + wave + .04); }
  const geometry = new THREE.BufferGeometry(); geometry.setAttribute('position', new THREE.Float32BufferAttribute(lines, 3)); group.add(new THREE.LineSegments(geometry, new THREE.LineBasicMaterial({ color: 0x8dada3, transparent: true, opacity: .14 })));
  return group;
}
function disposeObject(object: THREE.Object3D) { const materials = new Set<THREE.Material>(), geometries = new Set<THREE.BufferGeometry>(); object.traverse(item => { if (item instanceof CSS2DObject) item.element.remove(); const drawable = item as THREE.Mesh; if (drawable.geometry) geometries.add(drawable.geometry); if (drawable.material) for (const material of Array.isArray(drawable.material) ? drawable.material : [drawable.material]) materials.add(material); }); materials.forEach(material => { (material as THREE.MeshBasicMaterial).map?.dispose(); material.dispose(); }); geometries.forEach(geometry => geometry.dispose()); }
function fallbackScene(host: HTMLElement, callbacks: SceneCallbacks): SceneController {
  host.classList.add('atlas-fallback');
  function update(state: SceneState) {
    host.replaceChildren(); const note = document.createElement('p'); note.textContent = '3D rendering is unavailable. This accessible domain index supports selection and editing.'; host.append(note);
    function button(parent: HTMLElement, id: string, name: string) { const element = document.createElement('button'); element.textContent = name; element.dataset.subjectId = id; element.setAttribute('aria-pressed', String(state.selectedId === id)); element.addEventListener('click', () => callbacks.select(id)); parent.append(element); }
    for (const context of state.project.contexts.filter(item => !state.scopeId || item.id === state.scopeId)) { const section = document.createElement('section'); button(section, context.id, context.name); for (const concept of state.project.concepts.filter(item => item.contextId === context.id && (!state.aggregateId || item.id === state.aggregateId || item.ownerId === state.aggregateId))) button(section, concept.id, `${concept.name} · ${concept.kind.replaceAll('_', ' ')}`); host.append(section); }
    for (const relationship of state.project.relationships) button(host, relationship.id, relationship.label);
  }
  return { update, focus(id) { Array.from(host.querySelectorAll<HTMLButtonElement>('button')).find(button => button.dataset.subjectId === id)?.focus(); }, overview() { host.scrollTop = 0; }, topView() { host.scrollTop = 0; }, zoom() {}, dispose() { host.remove(); }, screenPosition() { return null; } };
}
