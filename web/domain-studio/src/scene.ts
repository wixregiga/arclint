import * as THREE from 'three';
import { OrbitControls } from 'three/addons/controls/OrbitControls.js';
import { CSS2DObject, CSS2DRenderer } from 'three/addons/renderers/CSS2DRenderer.js';
import { architectureImportsForSelection, type ArchitectureSubject } from './architecture-evidence';
import type { Concept, DomainContext, DomainProject, SceneCallbacks, SceneController, SceneState } from './contracts';
import { conceptContracts, relationshipDescription } from './model-evidence';
import { projectView, type FocusView } from './view-state';
import './scene.css';

const CHALK = 0xe9eff1, GRAPHITE = 0x233e4b, STONE = 0xb9cecf, ACCENT = 0xb68032;
const PAPER = 0xf8f5ed, PAPER_EDGE = 0xd6d6c9;
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
  const zoneKey=document.createElement('div');zoneKey.className='scene-zone-key';zoneKey.setAttribute('aria-label','File Zone colors');zoneKey.hidden=true;host.append(zoneKey);
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
  let priorInventory = '';
  const codeFrames: THREE.Group[] = [];
  let renderedDependencies = 0, scopedDependencies = 0;
  const layersVisible = () => state?.visibility?.layers ?? ['structure', 'governance'].includes(state?.lens ?? '');
  const dependenciesVisible = () => state?.visibility?.dependencies ?? true;
  const activeLayer = () => state?.architecture?.layers.find(rule => rule.id === state?.selectedLayerRule) ?? state?.architecture?.layers[0];
  const visibleLayerZones = () => activeLayer()?.zones.filter(zone => !state?.hiddenLayerZones?.includes(zone)) ?? [];
  const spread = () => THREE.MathUtils.clamp(state?.layerSpread ?? .65, 0, 1);
  const rememberedCameras = new Map<string, { position: THREE.Vector3; target: THREE.Vector3 }>();
  const locations = new Map<string, THREE.Vector3>(), groups = new Map<string, THREE.Group>();
  const worldPositions = new Map<string, THREE.Vector3>();
  const displayOffsets = new Map<string, THREE.Vector2>();
  const zoneColors = new Map<string, number>();
  const zoneColor = (name: string) => zoneColors.get(name) ?? CONTEXT;
  let contextProjection: { x:number; z:number; scaleX:number; scaleZ:number } | null = null;
  const displayPoint=(position:readonly number[],id?:string)=>new THREE.Vector3((contextProjection?contextProjection.x+(position[0]-contextProjection.x)*contextProjection.scaleX:position[0])+(id?displayOffsets.get(id)?.x ?? 0:0),groundY,(contextProjection?contextProjection.z+(position[2]-contextProjection.z)*contextProjection.scaleZ:position[2])+(id?displayOffsets.get(id)?.y ?? 0:0));
  const picks: THREE.Object3D[] = [], textObjects: CSS2DObject[] = [];
  const hoverLabels = new Map<string, HTMLElement>();
  const surfaceActions = new Map<string, () => void>();
  let annotationActions = 0;
  let hovered: string | null = null;
  let groundY = 0;
  let contextFrame: THREE.Group | null = null;
  let evidenceFrame: THREE.Group | null = null;
  let pressed: { x: number; y: number; id: string | null } | null = null;
  let drag: { id: string; plane: THREE.Plane; offset: THREE.Vector3; start: THREE.Vector3; moved: boolean } | null = null;
  const raycaster = new THREE.Raycaster(), pointer = new THREE.Vector2();
  const changed = () => { needsRender = true; }; controls.addEventListener('change', changed);

  function resize() {
    if (host.clientWidth < 2 || host.clientHeight < 2) return;
    const width = Math.max(1, host.clientWidth), height = Math.max(1, host.clientHeight);
    renderer.setSize(width, height); labels.setSize(width, height); camera.aspect = width / height; camera.updateProjectionMatrix(); needsRender = true; if (state) { update(state); overview(false, true); }
  }
  const observer = new ResizeObserver(resize); observer.observe(host); resize();
  const shellObserver = new ResizeObserver(() => { if (state && !disposed) overview(false, true); });
  for (const element of document.querySelectorAll('.place-header, .place-orientation, .governance-access, #build-bar, .view-pages, #inspector, #architecture-tools, #architecture-reading')) shellObserver.observe(element);
  function cast(event: PointerEvent) { const rect = canvas.getBoundingClientRect(); pointer.set((event.clientX - rect.left) / rect.width * 2 - 1, -(event.clientY - rect.top) / rect.height * 2 + 1); raycaster.setFromCamera(pointer, camera); }
  function pick(event: PointerEvent): string | null { cast(event); const hit = raycaster.intersectObjects(picks, false)[0]?.object; return hit?.userData.sceneActionId ?? hit?.userData.subjectId ?? null; }
  function activate(id: string | null) {
    if (id && surfaceActions.has(id)) { surfaceActions.get(id)!(); return; }
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
    if (drag) { const position = groups.get(drag.id)?.position,offset=displayOffsets.get(drag.id) ?? new THREE.Vector2(); if (position && drag.moved) callbacks.move(drag.id, [round(contextProjection?contextProjection.x+(position.x-offset.x-contextProjection.x)/contextProjection.scaleX:position.x), state?.project.concepts.find(subject => subject.id === drag!.id)?.position[1] ?? 0, round(contextProjection?contextProjection.z+(position.z-offset.y-contextProjection.z)/contextProjection.scaleZ:position.z)]); drag = null; controls.enabled = true; pressed = null; host.classList.remove('is-dragging'); return; }
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
    button.dataset.sceneFocus = JSON.stringify(['subject', subject.id, button.dataset.nodeState]);
    const inspectionContext=state?.architecture?.contexts.find(item=>item.id===(context?subject.id:subject.contextId));
    const inspectionEvidence=context?inspectionContext:inspectionContext?.subjects.find(item=>item.id===subject.id);
    button.dataset.inspectionState=inspectionPhase(inspectionEvidence);
    button.setAttribute('aria-pressed', String(state?.selectedId === subject.id));
    const name = document.createElement('span'); name.className = 'study-name'; name.textContent = subject.name; button.append(name);
    const detail = document.createElement('small'); detail.textContent = context ? `${state!.project.concepts.filter(term => term.contextId === subject.id).length} recorded entries · Open` : subject.kind === 'unclassified' ? 'Open question · kind unresolved' : subject.kind.replaceAll('_', ' '); button.append(detail);
    if (layersVisible()) {
      const contextEvidence = state?.architecture?.contexts.find(item => item.id === (context ? subject.id : subject.contextId));
      const evidence = context ? contextEvidence : contextEvidence?.subjects.find(item=>item.id===subject.id);
      if (evidence?.zones.length) {
        const zones = document.createElement('span'); zones.className = 'study-context-zones';
        button.dataset.zoneEvidence = `Located files in ${evidence.zones.join(' · ')}`;
        if(context) zones.textContent=evidence.zones.slice(0,3).join(' · ')+(evidence.zones.length>3?` · +${evidence.zones.length-3} in Details`:'');
        else { zones.classList.add('study-entry-zone-names'); zones.textContent=button.dataset.zoneEvidence; }
        button.append(zones);
      }
    }
    const diagnosticCount=!state?.checking && !state?.checkError ? inspectionEvidence?.diagnostics.length ?? 0 : 0;
    button.setAttribute('aria-label', `${subject.name}, ${context ? 'enter context' : detail.textContent}${button.dataset.zoneEvidence ? ` · ${button.dataset.zoneEvidence}` : ''}${diagnosticCount?` · ${diagnosticCount} returned findings`:''}`);
    if (context) { button.dataset.recordedTerms = String(state!.project.concepts.filter(term => term.contextId === subject.id).length); button.title = 'Bounded context · Settlement buildings represent recorded entries; the atlas position is a display projection.'; }
    if (view?.externalIds.includes(subject.id)) { const external = document.createElement('span'); external.className = 'study-external'; external.textContent = `↗ ${state?.project.contexts.find(context => context.id === ('contextId' in subject ? subject.contextId : subject.id))?.name ?? 'External context'}`; external.title = 'Recorded subject outside the current context'; button.append(external); }
    button.addEventListener('click', event => { event.stopPropagation(); if (!event.shiftKey) activate(subject.id); });
    button.addEventListener('pointerdown', event => { if (event.shiftKey) beginDrag(event, subject.id); });
    hoverLabels.set(subject.id, button);
    const object = new CSS2DObject(button); textObjects.push(object); return object;
  }
  function actor(subject: Subject, ghost = false) {
    const context = !('kind' in subject), group = new THREE.Group(); group.position.copy(context && worldPositions.has(subject.id) ? worldPositions.get(subject.id)! : displayPoint(subject.position,subject.id));
    const count = context ? state!.project.concepts.filter(term => term.contextId === subject.id).length : 0;
    const extent = regionExtent(count);
    const authoredColor=context?subject.color:state!.project.contexts.find(item=>item.id===subject.contextId)?.color;
    const tint=authoredColor && /^#[0-9a-f]{6}$/i.test(authoredColor)?parseInt(authoredColor.slice(1),16):CONTEXT;
    const body = context ? contextForm(state!.project.concepts.filter(term => term.contextId === subject.id), state!.project,tint) : conceptForm(subject.kind, view?.anchorId === subject.id,tint);
    if (!context && view?.anchorId !== subject.id) { const contracts=conceptContracts(state!.project,subject); protectionMarks(body,contracts.invariants.length,contracts.assertions.length); }
    group.add(body);
    if(!context && view?.level==='context' && layersVisible()){
      const evidence=state?.architecture?.contexts.find(item=>item.id===subject.contextId)?.subjects.find(item=>item.id===subject.id);
      const zones=evidence?.zones.filter(zone=>visibleLayerZones().includes(zone)).slice(0,3) ?? [];zones.forEach((zone,index)=>{const width=4.6/zones.length;const plate=add(body,new THREE.BoxGeometry(width-.12,.08,.5),zoneColor(zone),-2.3+width*(index+.5),.25,1.95);plate.userData.declaredZone=zone;});
    }
    if(!context && view?.anchorId!==subject.id && !ghost){
      const evidence=state?.architecture?.contexts.find(item=>item.id===subject.contextId)?.subjects.find(item=>item.id===subject.id);
      const phase=inspectionPhase(evidence);
      if(evidence?.anchors.some(anchor=>anchor.membership==='reported')){inspectionNotice(body,2.4,1.8,phase,.55);if(phase==='active')inspectionCrack(body);}
    }
    if(context && !ghost){
      const evidence=state?.architecture?.contexts.find(item=>item.id===subject.id);
      if(evidence?.diagnostics.length && !state?.checking && !state?.checkError)inspectionNotice(body,2.4,extent.depth*.4,inspectionPhase(evidence),.8);
    }
    if(!context && view?.level==='context')body.scale.setScalar(1.35);
    markPickables(body, subject.id);
    if (ghost) body.traverse(object => { if (object instanceof THREE.Mesh) { const material = object.material as THREE.MeshStandardMaterial; material.color.setHex(ACCENT); material.wireframe = true; material.transparent = true; material.opacity = .55; object.castShadow = false; } });
    if (state?.selectedId === subject.id) {
      add(group, new THREE.BoxGeometry(context ? 5 : 4.8, .1, .13), ACCENT, 0, .17, context ? extent.depth / 2 : 1.9);
    }
    const title = label(subject, ghost); title.position.set(0, .25, context ? extent.depth * .6 : view?.level==='context'?5.3:4.3); group.add(title);
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
        const line = new THREE.Line(new THREE.BufferGeometry().setFromPoints([displayPoint(old.position,subject.id).add(new THREE.Vector3(0,.2,0)), group.position.clone().add(new THREE.Vector3(0, .2, 0))]), new THREE.LineDashedMaterial({ color: ACCENT, dashSize: .35, gapSize: .25, transparent: true, opacity: .8 })); line.computeLineDistances(); world.add(line);
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
      const material = new THREE.LineDashedMaterial({ color: relation.id === state.selectedId ? ACCENT : GRAPHITE, transparent: true, opacity: .5, dashSize: .35, gapSize: .3 });
      const line = new THREE.Line(new THREE.BufferGeometry().setFromPoints(curve.getPoints(30)), material); line.computeLineDistances(); line.userData.recordedDomainRelation = relation.id; world.add(line);
      const arrow = (t: number, reverse: boolean) => { const mesh = piece(new THREE.ConeGeometry(.25, .65, 3), relation.id === state!.selectedId ? ACCENT : GRAPHITE); mesh.position.copy(curve.getPoint(t)); mesh.quaternion.setFromUnitVectors(new THREE.Vector3(0, 1, 0), curve.getTangent(t).normalize().multiplyScalar(reverse ? -1 : 1)); mesh.userData.subjectId = relation.id; world.add(mesh); picks.push(mesh); };
      if (semantics.direction !== 'none') arrow(.94, false); if (semantics.direction === 'both') arrow(.06, true);
      if (!relation.derived) locations.set(relation.id, middle);
    }
  }
  function membershipFrames() {
    // Entering a context opens its interior. No enclosing district rim survives inside it.
    contextFrame = null;
  }
  function updateZoneKey() {
    zoneKey.replaceChildren(); zoneKey.hidden = true;
    if (!state) return;
    const layer = activeLayer();
    const names = layersVisible() ? visibleLayerZones() : [];
    if (names.length) {
      const title = document.createElement('b'); title.textContent = 'Layers · higher → lower';
      title.title = 'The selected Rule declares this order. Filled swatches have located source files in this view; hollow swatches have no established file membership here.';
      zoneKey.append(title);
      const evidence = state.architecture;
      const anchors = evidence?.contexts.flatMap(context => context.subjects)
        .filter(subject => view?.level === 'detail' ? subject.id === view.anchorId : view?.ids.includes(subject.id))
        .flatMap(subject => subject.anchors) ?? [];
      const unresolved = view?.level === 'world' ? evidence?.observedImports.state !== 'reported'
        : !evidence?.linked || anchors.some(anchor => anchor.membership !== 'reported');
      names.forEach((name, index) => {
        if (index) { const arrow = document.createElement('span'); arrow.className = 'scene-layer-order'; arrow.textContent = '→'; arrow.setAttribute('aria-hidden', 'true'); zoneKey.append(arrow); }
        const paths = view?.level === 'world' ? evidence?.zones.find(zone => zone.name === name)?.observedPaths ?? []
          : anchors.filter(anchor => anchor.membership === 'reported' && anchor.zones.includes(name)).map(anchor => anchor.path);
        const count = new Set(paths).size;
        const item = document.createElement(count ? 'button' : 'span'), swatch = document.createElement('i');
        const color = `#${zoneColor(name).toString(16).padStart(6,'0')}`;
        swatch.style.background = count ? color : 'transparent'; swatch.style.borderColor = color;
        item.dataset.zoneName = name; item.dataset.zoneColor = color;
        item.dataset.layerOccupied = String(count > 0); item.dataset.layerFileCount = String(count);
        item.dataset.layerOccupancy = count ? 'occupied' : unresolved ? 'unresolved' : 'empty';
        item.title = `${name}: ${count} ${view?.level === 'world' ? 'observed' : 'located'} source file${count === 1 ? '' : 's'} in this view${unresolved ? ' so far; source membership remains unresolved' : ''}. This count does not establish context ownership, import permission, or that this Zone has no other code.`;
        if (count) {
          (item as HTMLButtonElement).type = 'button'; item.dataset.inspectZone = name; item.dataset.sceneFocus = JSON.stringify(['zone', name]);
          item.addEventListener('click', event => { event.stopPropagation(); callbacks.inspectZone?.(name); });
        } else item.setAttribute('role', 'img');
        item.setAttribute('aria-label', item.title);
        item.append(swatch, document.createTextNode(name)); zoneKey.append(item);
      });
      zoneKey.dataset.layerRule = layer?.id ?? '';
    }
    if (dependenciesVisible() && state.architecture) {
      const selection = architectureImportsForSelection(state.architecture, view?.level === 'detail' ? view.anchorId : null, view?.level === 'world' ? null : view?.contextId ?? null);
      host.dataset.dependencyResolution = selection.resolution;
      const caption = renderedDependencies ? selection.resolution === 'pending' ? `→ ${renderedDependencies} imports shown · resolving source files` : `→ ${renderedDependencies} of ${scopedDependencies} observed imports`
        : selection.resolution !== 'known' ? '' : scopedDependencies ? `${scopedDependencies} observed imports · full routes in Details`
        : view?.level === 'world' ? 'No internal imports reported' : 'No observed imports at these located files';
      if (caption) {
        const routes = document.createElement('span'); routes.className = 'scene-route-key'; routes.textContent = caption;
        routes.title = `${selection.reason} Solid arrows follow observed imports. Directory targets remain directories. Full evidence is available in Details.`;
        zoneKey.append(routes);
      }
    } else delete host.dataset.dependencyResolution;
    zoneKey.hidden = !zoneKey.childElementCount;
    const rect = host.getBoundingClientRect();
    const bottoms = [...document.querySelectorAll<HTMLElement>('#architecture-tools')].filter(element => !element.hidden && element.offsetParent !== null).map(element => element.getBoundingClientRect().bottom - rect.top);
    zoneKey.style.top = `${Math.max(100, ...bottoms) + 6}px`;
  }
  /** Labels stay attached to the exact record, source tile or Zone sheet they describe. */
  function annotation(parent: THREE.Group, title: string, detail: string, position: THREE.Vector3, action?: () => void, data: Record<string, string> = {}) {
    const interactive = !!action && groups.size + annotationActions < 8;
    const element = document.createElement(interactive ? 'button' : 'div');
    element.className = 'maquette-note';
    if (interactive) {
      (element as HTMLButtonElement).type = 'button'; annotationActions++; element.addEventListener('click', event => { event.stopPropagation(); action!(); });
      const key = `surface-${annotationActions}`; surfaceActions.set(key, action!);
      const surface = parent.children.filter((object): object is THREE.Mesh => object instanceof THREE.Mesh && !object.userData.sceneActionId).sort((a,b) => a.position.distanceToSquared(position)-b.position.distanceToSquared(position))[0];
      if (surface && surface.position.distanceTo(position) < 4) { surface.userData.sceneActionId = key; picks.push(surface); }
    }
    const heading = document.createElement('b'); heading.textContent = title; element.append(heading);
    if (detail) { const description = document.createElement('small'); description.textContent = detail; element.append(description); }
    Object.assign(element.dataset, data);
    const identity = data.inspectContract ? ['contract', data.contractOwner, data.contractKind, data.inspectContract]
      : data.inspectPath ? ['path', data.inspectPath]
      : data.inspectZone ? ['zone', data.inspectZone]
      : data.inspectRule ? ['rule', view?.anchorId, data.inspectRule]
      : data.reportState ? ['inspection', view?.anchorId ?? view?.selectedId]
      : ['note', title, detail];
    element.dataset.sceneFocus = JSON.stringify(identity);
    element.setAttribute('aria-label', `${title}${detail ? ` · ${detail}` : ''}`);
    element.dataset.annotationId = `note-${textObjects.length}`;
    const label = new CSS2DObject(element); label.position.copy(position); parent.add(label); textObjects.push(label);
    return element;
  }
  function recordLeaves(subject: Concept, group: THREE.Group) {
    const contracts = conceptContracts(state!.project, subject);
    const leaves = [
      ...contracts.invariants.map(contract => ({ ...contract, kind: 'invariant' as const, detail: contract.statement })),
      ...contracts.assertions.map(contract => ({ ...contract, kind: 'assertion' as const, detail: `After ${contract.operation} · ${contract.statement}` })),
    ];
    // Keep an operation contract visible alongside an invariant when both are recorded.
    const chosen = contracts.assertions.length && contracts.invariants.length ? [leaves[0], leaves.find(leaf => leaf.kind === 'assertion')!] : leaves.slice(0, 2);
    const left = -5.8;
    if (!chosen.length) return;
    chosen.forEach((contract, index) => {
      const z = index ? 3.9 : -2.1;
      guardPost(group,left,z,contract.kind==='assertion',1.7);
      const tab = annotation(group, contract.key, contract.kind === 'invariant' ? 'Recorded invariant ↗' : `After ${'operation' in contract ? contract.operation : ''} ↗`, new THREE.Vector3(left, 3.8, z), () => callbacks.inspectContract?.(subject.id, contract.key, contract.kind), { inspectContract: contract.key, contractKind: contract.kind, contractOwner: subject.id });
      tab.setAttribute('aria-label',`${contract.kind} · ${contract.key} · ${contract.statement}${'operation' in contract ? ` · After ${contract.operation}` : ''}`);
    });
    if (leaves.length > chosen.length) { const next = leaves.find(leaf => !chosen.includes(leaf))!; annotation(group, `+${leaves.length - chosen.length} recorded contracts`, 'Review another recorded promise', new THREE.Vector3(left, 1, 6.1), () => callbacks.inspectContract?.(subject.id,next.key,next.kind), {inspectContract:next.key,contractKind:next.kind,contractOwner:subject.id}); }
    group.userData.recordLeaves = chosen.length;
  }
  function inspectionPhase(evidence?: Pick<ArchitectureSubject,'anchors'|'diagnostics'>) {
    if(state?.checking)return 'checking';
    if(state?.checkError)return 'unavailable';
    if(state?.architecture?.evaluation.state!=='reported')return 'pending';
    if(!evidence?.anchors.some(anchor=>anchor.membership==='reported'))return 'unknown';
    const diagnostic=evidence.diagnostics.find(item=>item.status==='active') ?? evidence.diagnostics[0];
    return diagnostic?(diagnostic.status ?? diagnostic.kind):'reported';
  }
  function architectureCutaway() {
    evidenceFrame = null;
    if (!state || !view || view.level!=='detail') return;
    const subject = state.project.concepts.find(concept => concept.id === view!.anchorId);
    if(!subject)return;
    const home = view.subjectContextId ?? view.contextId;
    const group = new THREE.Group(); group.position.copy(displayPoint(subject.position)); world.add(group); evidenceFrame = group;
    const contextEvidence = state.architecture?.contexts.find(item => item.id === home);
    const subjectEvidence = contextEvidence?.subjects.find(item => item.id === subject.id);
    recordLeaves(subject, group);
    {
      const evaluation=state.architecture?.evaluation,diagnostics=subjectEvidence?.diagnostics ?? [];
      const diagnostic=state.checking || state.checkError?undefined:diagnostics.find(item=>item.status==='active') ?? diagnostics[0];
      const phase=inspectionPhase(subjectEvidence);
      inspectionNotice(group,2.7,1.7,phase,.8);
      if(phase==='active')inspectionCrack(group);
      const captions:Record<string,string>={checking:'Inspecting…',unavailable:'Inspection unavailable ↗',pending:'Check code ↗',unknown:'Code link unknown ↗',active:`${diagnostics.filter(item=>item.status==='active').length} active finding${diagnostics.filter(item=>item.status==='active').length===1?'':'s'} ↗`,baselined:'Acknowledged debt ↗',suppressed:'Suppressed finding ↗',operational:'Inspection incomplete ↗',coverage:'Coverage gap ↗',reported:'Report received ↗'};
      const action=phase==='checking'?undefined:phase==='pending'?()=>callbacks.checkCode?.():diagnostic?.ruleId?()=>callbacks.inspectRule?.(diagnostic.ruleId!):()=>callbacks.inspectReport?.();
      const data:Record<string,string>={reportState:evaluation?.state ?? 'not-run',inspectionState:phase,...(diagnostic?{diagnosticStatus:diagnostic.status ?? diagnostic.kind,...(diagnostic.ruleId?{inspectRule:diagnostic.ruleId}:{})}:{})};
      if(phase!=='unknown' && phase!=='reported'){
      const note=annotation(group,captions[phase] ?? 'Review inspection ↗','',new THREE.Vector3(2.7,2.7,2.6),action,data);note.classList.add('inspection-caption');
      note.setAttribute('aria-label',phase==='checking'?'Inspecting code. No current result yet.':phase==='unavailable'?`Inspection unavailable. ${state.checkError}. Open report details.`:phase==='pending'?'Not checked. Run Check code.':phase==='unknown'?'Code association unknown. No finding state can be assigned to this building.':diagnostic?`${diagnostic.status ?? diagnostic.kind} · ${diagnostic.ruleId ?? ''} · ${diagnostic.path ?? ''}${diagnostic.line?`:${diagnostic.line}`:''} · ${diagnostic.message}`:'Report received. No returned findings for located paths; this does not establish every Rule passed.');
      }
    }
  }

  /** Code structure is attached to observed paths; it never assigns a context to an architecture tier. */
  function codeStructure() {
    renderedDependencies = 0; scopedDependencies = 0;
    const evidence = state?.architecture;
    if (!state || !view || !evidence || (!layersVisible() && !dependenciesVisible())) return;
    const imports = architectureImportsForSelection(evidence, view.level === 'detail' ? view.anchorId : null, view.level === 'world' ? null : view.contextId);
    const rule = activeLayer(), order = visibleLayerZones();
    const observed = evidence.observedImports;
    type Endpoint = { id: string; path: string; kind: 'file' | 'directory'; zones: string[]; paths: string[]; position: THREE.Vector3; owner?: string };
    const endpoints = new Map<string, Endpoint>();
    const visibleEvidence = evidence.contexts.flatMap(context => context.subjects).filter(subject => view!.level === 'detail' ? subject.id === view!.anchorId : view!.ids.includes(subject.id));
    const located = [...new Map(visibleEvidence.flatMap(subject => subject.anchors.filter(anchor => anchor.membership === 'reported').map(anchor => [anchor.path, { anchor, owner: subject.id }] as const))).values()];
    const locatedPaths = new Set(located.map(item => item.anchor.path));
    const edges = view.level === 'world' ? imports.edges : imports.edges.filter(edge => locatedPaths.has(edge.sourcePath) || edge.targetKind === 'file' && locatedPaths.has(edge.targetPath));
    scopedDependencies = edges.length;
    if (!layersVisible() && (observed.state !== 'reported' || !edges.length)) return;
    const folder = (path: string) => path.includes('/') ? path.slice(0, path.lastIndexOf('/')) : '.';
    const bounds = new THREE.Box3();
    for (const group of groups.values()) bounds.expandByPoint(group.position);
    if (bounds.isEmpty()) bounds.set(new THREE.Vector3(-6, 0, -4), new THREE.Vector3(6, 0, 4));
    const center = bounds.getCenter(new THREE.Vector3());
    let selectedEdges = edges;
    if (view.level === 'world') {
      // Package bundles are a repository code section, not edges between domain towns.
      const bundles = new Map<string, { paths: Set<string>; zones: Set<string>; weight: number }>();
      const remember = (path: string, zones: string[], sourcePath?: string) => {
        const bundle = bundles.get(path) ?? { paths: new Set<string>(), zones: new Set<string>(), weight: 0 };
        if (sourcePath) bundle.paths.add(sourcePath); zones.forEach(zone => bundle.zones.add(zone)); bundle.weight++; bundles.set(path, bundle);
      };
      if (dependenciesVisible()) for (const edge of edges) {
        remember(folder(edge.sourcePath), edge.sourceZones, edge.sourcePath);
        remember(edge.targetKind === 'directory' ? edge.targetPath : folder(edge.targetPath), edge.targetZones, edge.targetKind === 'file' ? edge.targetPath : undefined);
      }
      if (layersVisible() && observed.state === 'reported') for (const file of observed.files.filter(file => file.zones.some(zone => order.includes(zone)))) remember(folder(file.path), file.zones, file.path);
      const ranked = [...bundles].sort((a,b) => b[1].weight - a[1].weight || a[0].localeCompare(b[0]));
      const selected: typeof ranked = [];
      if (layersVisible()) for (const zone of order) {
        const representative = ranked.find(([path, bundle]) => bundle.zones.has(zone) && !selected.some(([chosen]) => chosen === path));
        if (representative && selected.length < 3) selected.push(representative);
      }
      for (const bundle of ranked) if (selected.length < 3 && !selected.some(([path]) => path === bundle[0])) selected.push(bundle);
      selected.forEach(([path, bundle], index) => {
        const position = new THREE.Vector3(center.x + (index - (selected.length - 1) / 2) * 15, .25, bounds.min.z - 10);
        endpoints.set(path, { id: path, path, kind: 'directory', zones: [...bundle.zones], paths: [...bundle.paths], position });
      });
    } else {
      // A visible source port records an association with this exact file, not domain ownership.
      const eligible = layersVisible() ? located : located.filter(({anchor}) => edges.some(edge => edge.sourcePath === anchor.path || edge.targetKind === 'file' && edge.targetPath === anchor.path));
      const selected = eligible.slice(0, view.level === 'detail' ? 2 : 3);
      selected.forEach(({anchor, owner}, index) => {
        const actorPosition = locations.get(owner) ?? center;
        const position = view!.level === 'detail' ? new THREE.Vector3(actorPosition.x + 6.8, .25, actorPosition.z - 3 + index * 5.5)
          : actorPosition.clone().add(new THREE.Vector3(3.8, .25, 1.4));
        endpoints.set(anchor.path, { id: anchor.path, path: anchor.path, kind: 'file', zones: anchor.zones, paths: [anchor.path], position, owner });
      });
      const represented = new Set(selected.map(item => item.anchor.path));
      selectedEdges = edges.filter(edge => represented.has(edge.sourcePath) || edge.targetKind === 'file' && represented.has(edge.targetPath));
      const destinations = new Map<string, { kind: 'file' | 'directory'; zones: string[]; weight: number }>();
      if (dependenciesVisible()) for (const edge of selectedEdges) {
        if (!represented.has(edge.sourcePath)) {
          const previous = destinations.get(edge.sourcePath);
          destinations.set(edge.sourcePath, { kind: 'file', zones: edge.sourceZones, weight: (previous?.weight ?? 0) + 1 });
        }
        if (!represented.has(edge.targetPath) && edge.targetKind !== 'unresolved') {
          const previous = destinations.get(edge.targetPath);
          destinations.set(edge.targetPath, { kind: edge.targetKind, zones: edge.targetZones, weight: (previous?.weight ?? 0) + 1 });
        }
      }
      [...destinations].sort((a,b) => b[1].weight - a[1].weight || a[0].localeCompare(b[0])).slice(0, 2).forEach(([path, target], index) => {
        const position = view!.level === 'detail' ? new THREE.Vector3(center.x + 8, .25, center.z + 8 + index * 5.5)
          : new THREE.Vector3(center.x + (index ? 7 : -7), .25, bounds.min.z - 9);
        endpoints.set(path, { id: path, path, kind: target.kind, zones: target.zones, paths: target.kind === 'file' ? [path] : [], position });
      });
    }
    // An endpoint needs visible evidence: an occupied shown layer or an actual
    // drawn import. Dependency-only mode never leaves disconnected source tiles.
    const connected = new Set<string>();
    if (dependenciesVisible()) for (const edge of selectedEdges) {
      const from = view.level === 'world' ? folder(edge.sourcePath) : edge.sourcePath;
      const to = view.level === 'world' ? edge.targetKind === 'directory' ? edge.targetPath : folder(edge.targetPath) : edge.targetPath;
      if (from !== to && endpoints.has(from) && endpoints.has(to)) { connected.add(from); connected.add(to); }
    }
    for (const [id, endpoint] of endpoints) {
      if (!(layersVisible() && endpoint.zones.some(zone => order.includes(zone))) && !connected.has(id)) endpoints.delete(id);
    }
    const endpointLabels = [...endpoints.values()].map(endpoint => endpoint.path);
    for (const endpoint of endpoints.values()) {
      const group = new THREE.Group(); group.position.copy(endpoint.position); group.userData.codeEndpoint = endpoint.id;
      world.add(group); codeFrames.push(group);
      const memberZones = order.filter(zone => endpoint.zones.includes(zone));
      const tiers = layersVisible() ? memberZones : [];
      const scale = view.level === 'context' && endpoint.owner ? .68 : 1;
      const width = (view.level === 'detail' ? 4.8 : 7) * scale, depth = (view.level === 'detail' ? 2.4 : 3.2) * scale;
      tiers.forEach(zone => {
        const y = .12 + (order.length - order.indexOf(zone) - 1) * (.18 + spread() * 2.4);
        const shelf = add(group, new THREE.BoxGeometry(width + .45, .15, depth + .35), zoneColor(zone), 0, y, 0);
        shelf.userData.layerZone = zone; shelf.userData.layerRule = rule?.id; shelf.userData.observedPaths = endpoint.paths;
        // Overlaps are several memberships of one source, never copies of its domain subject.
        const tab = add(group, new THREE.BoxGeometry(.7, .14, .5), zoneColor(zone), width / 2 + .2, y, depth / 2 + .1);
        tab.userData.layerZone = zone;
      });
      const top = .4 + (tiers.length ? order.length - order.indexOf(tiers[0]) - 1 : 0) * (.18 + spread() * 2.4);
      const tile = add(group, new THREE.BoxGeometry(width, .3, depth), GRAPHITE, 0, top, 0);
      tile.userData.sourcePath = endpoint.path; tile.userData.dependencyEndpoint = endpoint.kind;
      if (endpoint.kind === 'directory') {
        add(group, new THREE.BoxGeometry(width * .35, .15, .55), STONE, -width * .23, top + .24, -depth / 2 + .2);
        if (endpoint.paths.length > 1) add(group, new THREE.BoxGeometry(width * .8, .12, depth * .7), STONE, .25, top + .23, .15);
      }
      endpoint.position.y += top + .25;
      // Context ports can be read on selection; avoid a second layer of labels over eight entries.
      const showLabel = view.level !== 'context' || !endpoint.owner;
      if (showLabel) {
        const title = sourceName(endpoint.path, endpointLabels);
        const detail = endpoint.kind === 'directory' ? `Source directory${endpoint.paths.length ? ` · ${endpoint.paths.length} files` : ''}` : 'Located source';
        const note = annotation(group, title, detail, new THREE.Vector3(0, top + .7, depth / 2 + .6), () => callbacks.inspectPath?.(endpoint.path), { inspectPath: endpoint.path, dependencyEndpoint: endpoint.kind, layerZones: tiers.join(',') });
        note.classList.add('code-port-label'); if (tiers.length) note.dataset.layerZone = tiers[0]; note.dataset.codeSection = view.level === 'world' ? 'repository' : 'located-source'; note.title = `${endpoint.path}${endpoint.zones.length ? ` · Zones: ${endpoint.zones.join(', ')}` : ''}`;
        note.setAttribute('aria-label', `${endpoint.path} · ${detail}${tiers.length ? ` · Declared layers: ${tiers.join(', ')}` : ''}`);
      } else {
        const ownerLabel = hoverLabels.get(endpoint.owner!);
        if (ownerLabel) { ownerLabel.dataset.sourcePath = endpoint.path; ownerLabel.dataset.layerZones = tiers.join(','); if (tiers.length) ownerLabel.dataset.layerZone = tiers[0]; ownerLabel.title = `${endpoint.path} · ${endpoint.zones.join(', ')}`; }
      }
    }
    if (!dependenciesVisible()) return;
    const groupedEdges = new Map<string, { from: Endpoint; to: Endpoint; count: number; source: string; target: string; kind: string }>();
    for (const edge of selectedEdges) {
      const from = endpoints.get(view.level === 'world' ? folder(edge.sourcePath) : edge.sourcePath);
      const to = endpoints.get(view.level === 'world' ? edge.targetKind === 'directory' ? edge.targetPath : folder(edge.targetPath) : edge.targetPath);
      if (!from || !to || from === to) continue;
      const key = `${from.id}→${to.id}`, previous = groupedEdges.get(key);
      groupedEdges.set(key, { from, to, count: (previous?.count ?? 0) + 1, source: edge.sourcePath, target: edge.targetPath, kind: edge.targetKind });
    }
    for (const route of groupedEdges.values()) {
      const start = route.from.position.clone(), end = route.to.position.clone();
      const direction = end.clone().sub(start).normalize(); start.addScaledVector(direction, 1.8); end.addScaledVector(direction, -1.8);
      const middle = start.clone().lerp(end, .5); middle.y += Math.min(3, start.distanceTo(end) * .12);
      const curve = new THREE.QuadraticBezierCurve3(start, middle, end);
      const line = new THREE.Line(new THREE.BufferGeometry().setFromPoints(curve.getPoints(28)), new THREE.LineBasicMaterial({ color: 0x376f87, transparent: true, opacity: .82 }));
      line.userData.observedImport = { source: route.source, target: route.target, targetKind: route.kind, count: route.count };
      const group = new THREE.Group(); group.add(line);
      const arrow = piece(new THREE.ConeGeometry(.38, .95, 3), 0x376f87); arrow.position.copy(curve.getPoint(.7)); arrow.quaternion.setFromUnitVectors(new THREE.Vector3(0,1,0), curve.getTangent(.7).normalize()); group.add(arrow);
      world.add(group); codeFrames.push(group); renderedDependencies += route.count;
      // Evidence hooks stay on an existing endpoint, without adding another selectable label.
      const note = textObjects.find(object => object.element.dataset.inspectPath === route.to.path)?.element;
      if (note) { note.dataset.dependencySource = route.source; note.dataset.dependencyTarget = route.target; note.dataset.dependencyTargetKind = route.kind; note.dataset.dependencyCount = String(route.count); }
    }
  }

  function update(next: SceneState) {
    if (disposed) return;
    const active = document.activeElement;
    const focusedIdentity = active instanceof HTMLElement && host.contains(active) ? active.dataset.sceneFocus : undefined;
    const fallbackScope = next.scopeId ?? (next.aggregateId ? next.project.concepts.find(item => item.id === next.aggregateId)?.contextId : null);
    state = next;
    const zoneNames=[...new Set([...(next.architecture?.zones.map(zone=>zone.name) ?? []),...(next.architecture?.contexts.flatMap(context=>context.zones) ?? [])])].sort();
    zoneColors.clear();zoneNames.forEach((name,index)=>zoneColors.set(name,zonePalette(index,zoneNames.length)));
    view = next.view ?? projectView(next.project, { scopeId: fallbackScope, selectedId: next.selectedId });
    // Defensive cap also covers malformed caller-provided views. No offstage geometry or proxy actors survives.
    view = { ...view, ids: [...new Set(view.ids)].slice(0, 8) };
    scene.remove(world); disposeObject(world); world = new THREE.Group(); scene.add(world); locations.clear(); groups.clear(); picks.length = 0; textObjects.length = 0; hoverLabels.clear(); surfaceActions.clear(); hovered = null; annotationActions = 0; codeFrames.length = 0;
    const current = new Map<string, Subject>([...next.project.contexts, ...next.project.concepts].map(subject => [subject.id, subject]));
    contextProjection=null;displayOffsets.clear();
    if(view.level==='context' && view.ids.length>1){
      const positions=view.ids.flatMap(id=>{const subject=current.get(id);return subject?[new THREE.Vector3(subject.position[0],0,subject.position[2])]:[];});
      const bounds=new THREE.Box3().setFromPoints(positions),size=bounds.getSize(new THREE.Vector3()),center=bounds.getCenter(new THREE.Vector3());
      const width=Math.max(220,host.clientWidth-(host.clientWidth<=650?36:next.visibility?.description === false?64:250)),height=Math.max(180,host.clientHeight-(host.clientWidth<=650?365:240));
      const aspect=THREE.MathUtils.clamp(width/height,.7,3.2);let scaleX=1,scaleZ=1;
      if(size.x>.1 && size.z>.1){const proposed=(aspect*(size.z*.72+6)-(size.z*.15+6))/(size.x*(.99-aspect*.1));if(proposed>=1)scaleX=THREE.MathUtils.clamp(proposed,1,3.8);else scaleZ=THREE.MathUtils.clamp(((.99-aspect*.1)*size.x+6*(1-aspect))/(size.z*(.72*aspect-.15)),1,2.8);}
      contextProjection={x:center.x,z:center.z,scaleX,scaleZ};
      // Separate rendered footprints in the entry camera's plane. These offsets
      // are presentation only and are subtracted before any saved drag update.
      const points=view.ids.flatMap(id=>{const subject=current.get(id);if(!subject)return [];const base=displayPoint(subject.position);return [{id,base,x:.989*base.x-.148*base.z,y:.122*base.x+.812*base.z}];});
      for(let iteration=0;iteration<40;iteration++){
        let overlap=false;
        for(let i=0;i<points.length;i++)for(let j=i+1;j<points.length;j++){
          const a=points[i],b=points[j],dx=b.x-a.x,dy=b.y-a.y;
          if(Math.abs(dx)>=13 || Math.abs(dy)>=11)continue;
          overlap=true;
          if(aspect>1){const step=(13-Math.abs(dx))*.5+.02,sign=dx<0?-1:1;a.x-=step*sign;b.x+=step*sign;}
          else{const step=(11-Math.abs(dy))*.5+.02,sign=dy<0?-1:1;a.y-=step*sign;b.y+=step*sign;}
        }
        if(!overlap)break;
      }
      const minX=Math.min(...points.map(point=>point.x)),maxX=Math.max(...points.map(point=>point.x)),minY=Math.min(...points.map(point=>point.y)),maxY=Math.max(...points.map(point=>point.y));
      const stageAspect=THREE.MathUtils.clamp(aspect*1.2,.65,3),spanX=maxX-minX,spanY=maxY-minY;
      const spreadX=Math.max(1,(stageAspect*(spanY+9)-9)/Math.max(spanX,1)),spreadY=Math.max(1,((spanX+9)/stageAspect-9)/Math.max(spanY,1));
      for(const point of points){const x=(minX+maxX)/2+(point.x-(minX+maxX)/2)*spreadX,y=(minY+maxY)/2+(point.y-(minY+maxY)/2)*spreadY;displayOffsets.set(point.id,new THREE.Vector2((.812*x+.148*y)/.821124-point.base.x,(-.122*x+.989*y)/.821124-point.base.z));}
    }
    worldPositions.clear();
    if(view.level==='world'){
      const centers=[...new Set(next.project.concepts.filter(term=>term.kind==='aggregate').map(term=>term.contextId))];
      if(centers.length===1 && view.ids.includes(centers[0])){
        // A sole aggregate-owning settlement is the model's civic landmark, not a dependency hub.
        worldPositions.set(centers[0],new THREE.Vector3(0,0,0));
        const others=view.ids.filter(id=>id!==centers[0]);others.forEach((id,index)=>{const angle=-Math.PI*.75+index*Math.PI*2/Math.max(others.length,1);worldPositions.set(id,new THREE.Vector3(Math.cos(angle)*30,0,Math.sin(angle)*25));});
      }
    }
    groundY = 0; floor.position.y = -.6;
    const groundColor=CHALK;(scene.background as THREE.Color).setHex(groundColor);(floor.material as THREE.MeshStandardMaterial).color.setHex(groundColor);
    for (const id of view.ids) { const subject = current.get(id); if (subject) actor(subject); else { const previous = next.baseline && [...next.baseline.project.contexts, ...next.baseline.project.concepts].find(item => item.id === id); if (previous) actor(previous, true); } }
    focusedRoutes(); membershipFrames(); architectureCutaway(); codeStructure(); updateZoneKey();
    host.dataset.viewLevel = view.level; host.dataset.visibleSubjects = String(groups.size); host.dataset.visibleMarks = String(groups.size + annotationActions); host.dataset.contextEnclosure = 'none'; host.dataset.lens = next.lens ?? 'meaning'; host.dataset.layersVisible = String(layersVisible()); host.dataset.dependenciesVisible = String(dependenciesVisible()); host.dataset.dependencyCount = String(renderedDependencies); host.dataset.dependencyTotal = String(scopedDependencies);
    needsRender = true; renderer.shadowMap.needsUpdate = true;
    // Navigation identity is independent from visibility, observations and camera distance.
    // A returned place recovers its last framing; changes in code geometry only
    // pull back when the new contents would otherwise be clipped.
    const place = `${view.level}|${view.contextId}|${view.anchorId ?? (view.level === 'detail' ? view.selectedId : '')}|${view.page}`;
    const inventory = `${layersVisible()}|${dependenciesVisible()}|${activeLayer()?.id ?? ''}|${visibleLayerZones().join(',')}|${spread()}|${evidenceFrame?.userData.recordLeaves ?? 0}|${codeFrames.flatMap(group => group.userData.codeEndpoint ? [`${group.userData.codeEndpoint}:${group.position.toArray().join(',')}`] : []).join('|')}`;
    if (place !== priorView || initialFrame) {
      if (priorView) rememberedCameras.set(priorView, { position: (flight?.to ?? camera.position).clone(), target: (flight?.target ?? controls.target).clone() });
      priorView = place;
      const remembered = rememberedCameras.get(place);
      if (remembered) { travel(remembered.position.clone(), remembered.target.clone()); overview(false, true); }
      else overview();
      initialFrame = false;
    } else if (inventory !== priorInventory) overview(false, true);
    priorInventory = inventory;
    // CSS2D nodes are replaced during evidence refreshes. Attach the new labels
    // before restoring focus, so an Enter arriving before the next frame still
    // reaches the same action. Never reclaim focus moved into another surface.
    if (focusedIdentity && (!document.activeElement || document.activeElement === document.body)) {
      const replacement = [...textObjects.map(object => object.element), ...zoneKey.querySelectorAll<HTMLButtonElement>('button[data-scene-focus]')].find(element => element instanceof HTMLButtonElement && element.dataset.sceneFocus === focusedIdentity);
      if (replacement) { labels.render(scene, camera); replacement.focus({ preventScroll: true }); }
    }
  }
  function travel(position: THREE.Vector3, target: THREE.Vector3) {
    needsRender = true;
    if (reducedMotion) { camera.position.copy(position); controls.target.copy(target); controls.update(); flight = null; host.dataset.cameraState = 'settled'; }
    else { flight = { from: camera.position.clone(), fromTarget: controls.target.clone(), to: position, target, started: performance.now() }; host.dataset.cameraState = 'moving'; }
  }
  function overview(overhead = false, onlyIfClipped = false) {
    const hostRect = host.getBoundingClientRect(), width = hostRect.width, height = hostRect.height;
    if (width < 2 || height < 2) return;
    const orientation = document.querySelector('.place-orientation')?.getBoundingClientRect();
    const mobile = width <= 650;
    const safeLeft = mobile ? 16 : state?.visibility?.description === false ? 32 : Math.min(width * .34, Math.max(32, (orientation?.right ?? 0) - hostRect.left + 24));
    const safeRight = width - (mobile ? 16 : 32);
    const topControls = [...document.querySelectorAll<HTMLElement>('.place-header, .governance-access, #architecture-tools, .scene-zone-key')].filter(element => !element.hidden && element.offsetParent !== null).map(element => element.getBoundingClientRect().bottom - hostRect.top);
    const bottomControls = [...document.querySelectorAll('#build-bar, .map-navigation, .view-pages')]
      .filter(element => (element as HTMLElement).offsetParent !== null).map(element => element.getBoundingClientRect())
      .filter(bounds => bounds.right > hostRect.left + safeLeft && bounds.left < hostRect.left + safeRight)
      .map(bounds => bounds.top - hostRect.top);
    const safe = {
      left: safeLeft,
      right: safeRight,
      top: mobile ? Math.max(110, ...topControls, (orientation?.bottom ?? 0) - hostRect.top + 18) : Math.max(100, ...topControls, state?.visibility?.description === false ? (orientation?.bottom ?? 0) - hostRect.top : 0) + 16,
      bottom: mobile ? Math.min(height - 215, ...bottomControls) - 12 : Math.min(height - 120, ...bottomControls) - 12,
    };
    // The nonmodal editor can retain a useful map beside it on desktop; phone editing owns the surface.
    const editor = document.querySelector<HTMLElement>('#inspector');
    if (!mobile && editor && !editor.hidden && editor.offsetParent !== null) {
      const rightEdge = editor.getBoundingClientRect().left - hostRect.left - 18;
      if (rightEdge - safe.left > 220) safe.right = Math.min(safe.right, rightEdge);
    }
    const reading = document.querySelector<HTMLElement>('#architecture-reading');
    if (reading && !reading.hidden && reading.offsetParent !== null) {
      const bounds = reading.getBoundingClientRect();
      if (mobile) safe.bottom = Math.min(safe.bottom, bounds.top - hostRect.top - 12);
      else if (bounds.left - hostRect.left - safe.left > 240) safe.right = Math.min(safe.right, bounds.left - hostRect.left - 18);
    }
    safe.bottom = Math.max(safe.top + 120, safe.bottom);
    // Shift the optical center into the unobscured stage. Orbit still revolves around the real model center.
    camera.setViewOffset(width, height, width / 2 - (safe.left + safe.right) / 2, height / 2 - (safe.top + safe.bottom) / 2, width, height);
    const points: THREE.Vector3[] = [];
    const domainBox = new THREE.Box3();
    for (const group of [...groups.values(), ...(contextFrame && view?.level === 'context' ? [contextFrame] : []), ...(evidenceFrame ? [evidenceFrame] : []), ...codeFrames]) {
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
    const inside = (probe: THREE.Camera) => points.every(point => {
      const p = point.clone().project(probe), x = (p.x + 1) * width / 2, y = (1 - p.y) * height / 2;
      return p.z > -1 && p.z < 1 && x >= safe.left + 34 && x <= safe.right - 34 && y >= safe.top + 10 && y <= safe.bottom - 34;
    });
    if (onlyIfClipped) {
      const current = camera.clone(); current.position.copy(flight?.to ?? camera.position); current.lookAt(flight?.target ?? controls.target); current.updateMatrixWorld(true);
      if (inside(current)) { needsRender = true; return; }
    }
    // A new place has an intentional entry angle; an overlay preserves the user's heading.
    const altitude = view?.level === 'detail' ? 1.35 : view?.level === 'context' ? 1.45 : 1.65;
    const direction = overhead ? new THREE.Vector3(0, 1, .001).normalize() : onlyIfClipped ? (flight?.to ?? camera.position).clone().sub(flight?.target ?? controls.target).normalize() : new THREE.Vector3(view?.level==='context'?.15:.3, altitude, 1).normalize();
    const probe = camera.clone();
    function fits(distance: number) {
      probe.position.copy(center).addScaledVector(direction, distance); probe.lookAt(center); probe.updateMatrixWorld(true);
      return inside(probe);
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
    const occupied = [...document.querySelectorAll<HTMLElement>('.place-header, .place-orientation, .governance-access, .focus-controls, #build-bar, .map-navigation, .view-pages, #keeper-action, #inspector, #onyx-support, #architecture-reading, #architecture-tools, .scene-zone-key')]
      .filter(element => !element.hidden && element.offsetParent !== null).map(element => element.getBoundingClientRect());
    const project = (position: THREE.Vector3) => { const p = position.clone().project(camera); return { x: rect.left + (p.x + 1) * rect.width / 2, y: rect.top + (1 - p.y) * rect.height / 2 }; };
    const markers = [...groups].map(([id, group]) => ({ id, point: project(group.position.clone().add(new THREE.Vector3(0, .45, 0))) }));
    for (const object of textObjects) { object.element.style.marginTop = '0px'; object.element.style.marginLeft = '0px'; }
    const objects = textObjects.map(object => ({ object, bounds: object.element.getBoundingClientRect() }))
      .sort((a, b) => {
        const priority = (object: CSS2DObject) => object.element.dataset.subjectId === state?.selectedId ? 3 : object.element.dataset.subjectKind === 'aggregate' ? 2 : object.element.dataset.subjectKind === 'entity' ? 1 : 0;
        return priority(b.object) - priority(a.object) || a.bounds.top - b.bounds.top || a.bounds.left - b.bounds.left;
      });
    const intersects = (a: DOMRect, b: DOMRect, gap = 5) => a.left < b.right + gap && a.right > b.left - gap && a.top < b.bottom + gap && a.bottom > b.top - gap;
    const fragment = document.createDocumentFragment();
    for (const { object, bounds } of objects) {
      if (bounds.width === 0 || bounds.top > rect.bottom || bounds.bottom < rect.top) continue;
      const id = object.element.dataset.subjectId ?? object.element.dataset.annotationId!, anchor = markers.find(marker => marker.id === id)?.point ?? project(object.getWorldPosition(new THREE.Vector3()));
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
    host.dataset.semanticDepth = view?.level === 'detail' || camera.position.distanceTo(controls.target) < 28 ? 'near' : 'overview';
    if (needsRender || moved || flying) { renderer.render(scene, camera); labels.render(scene, camera); keepLabelsSeparate(); host.dataset.drawCalls = String(renderer.info.render.calls); needsRender = false; }
  }
  animate();
  return { update, focus, overview, topView, zoom, screenPosition, dispose() { disposed = true; cancelAnimationFrame(frame); observer.disconnect(); shellObserver.disconnect(); controls.removeEventListener('change', changed); controls.dispose(); canvas.removeEventListener('pointerdown', pointerDown, true); canvas.removeEventListener('keydown', keyboard); canvas.removeEventListener('wheel', stopFlight); window.removeEventListener('pointermove', pointerMove); window.removeEventListener('pointerup', pointerUp); window.removeEventListener('pointercancel', pointerUp); disposeObject(scene); renderer.dispose(); host.remove(); } };
}

function round(value: number) { return Math.round(value * 100) / 100; }
function zonePalette(index:number,total:number){
  const palette=[0x397c9a,0xc9874d,0x839541,0x9870a8,0xc36473,0x508d77,0x8c684d,0x6583b4,0xad9758,0xa36339,0x6e887f,0x8e697c,0x3b6971,0xb28580,0x7774a5,0x5a847e,0xaca36b,0x538baf,0x9f594b,0x829666];
  return index<palette.length?palette[index]:new THREE.Color().setHSL(index/Math.max(total,1),.5,.45).getHex();
}
function sourceName(path: string, paths: string[]) {
  const parts=path.split('/');
  for(let length=1;length<=parts.length;length++){const suffix=parts.slice(-length).join('/');if(paths.filter(candidate=>candidate.split('/').slice(-length).join('/')===suffix).length===1)return suffix;}
  return path;
}
function piece(geometry: THREE.BufferGeometry, color = STONE) { const mesh = new THREE.Mesh(geometry, new THREE.MeshStandardMaterial({ color, roughness: .92, metalness: .02 })); mesh.castShadow = true; mesh.receiveShadow = true; return mesh; }
function add(group: THREE.Group, geometry: THREE.BufferGeometry, color: number, x: number, y: number, z: number) { const object = piece(geometry, color); object.position.set(x, y, z); group.add(object); return object; }
function regionExtent(count: number) {
  // Area grows with the complete recorded term count, never the current page of eight.
  const width = Math.sqrt(180 + count * 18); return { width, depth: width * .76 };
}
/** Miniatures are made from recorded entries; a civic hall appears only for an aggregate. */
function contextForm(terms: Concept[], project: DomainProject, tint=CONTEXT) {
  const group=new THREE.Group(), shown=terms.slice(0,32), aggregate=shown.find(term=>term.kind==='aggregate');
  const ordered=aggregate?[aggregate,...shown.filter(term=>term!==aggregate)]:shown;
  ordered.forEach((term,index)=>{
    const building=conceptForm(term.kind,false,tint), contracts=conceptContracts(project,term);
    protectionMarks(building,contracts.invariants.length,contracts.assertions.length);
    const count=ordered.length, columns=Math.max(1,Math.ceil(Math.sqrt(count))), row=Math.floor(index/columns),column=index%columns;
    let x=(column-(columns-1)/2)*3.2,z=(row-(Math.ceil(count/columns)-1)/2)*3.4;
    if(aggregate){if(index===0){x=0;z=0;}else{const ring=1+Math.floor((index-1)/8),angle=(index-1)*Math.PI/4;x=Math.cos(angle)*ring*5.1;z=Math.sin(angle)*ring*4.6;}}
    const scale=term.kind==='aggregate'?.78:.52;building.scale.setScalar(scale);building.position.set(x,0,z);group.add(building);
  });
  // An unrecorded city has a named empty site, without pretending it contains buildings.
  if(!terms.length)for(const x of [-2,2])add(group,new THREE.BoxGeometry(.7,.25,.7),STONE,x,.1,0);
  group.userData.cityEntries=shown.length;group.userData.recordedEntries=terms.length;
  return group;
}
function roof(group:THREE.Group,x:number,y:number,z:number,width:number,depth:number,color:number){
  const geometry=new THREE.CylinderGeometry(width*.6,width*.6,depth,3,1);geometry.rotateX(Math.PI/2);geometry.rotateZ(Math.PI);const mesh=add(group,geometry,color,x,y,z);mesh.scale.y=.62;return mesh;
}
function conceptForm(kind: Concept['kind'], open = false, tint=CONTEXT) {
  const group=new THREE.Group(), lift=open?1.5:0;
  const base=add(group,new THREE.BoxGeometry(5,.22,4.5),0xd5dfdd,0,.08,0);base.userData.buildingKind=kind;
  if(kind==='aggregate'){
    add(group,new THREE.BoxGeometry(4.7,.55,3.9),PAPER_EDGE,0,.45,0);
    add(group,new THREE.BoxGeometry(4.2,2.25,2.4),PAPER,0,1.85,-.55);
    for(const x of [-1.65,-.55,.55,1.65])add(group,new THREE.CylinderGeometry(.19,.22,2.8,12),PAPER,x,1.95,1.35);
    add(group,new THREE.BoxGeometry(4.75,.28,3.8),PAPER_EDGE,0,3.45+lift,0);
    roof(group,0,4.12+lift,0,4.15,4.15,tint);
    add(group,new THREE.BoxGeometry(.65,1.45,.25),GRAPHITE,0,1.72,.78);
  }else if(kind==='value_object'){
    add(group,new THREE.BoxGeometry(3.4,.32,2.9),PAPER_EDGE,0,.5,0);
    add(group,new THREE.BoxGeometry(2.7,.18,2.35),GRAPHITE,0,.75,0);
    const artifact=add(group,new THREE.CylinderGeometry(1.35,1.35,1.9,6),PAPER,0,1.85,0);artifact.rotation.y=Math.PI/6;
    for(const y of [1.03,2.67]){const seal=add(group,new THREE.CylinderGeometry(1.42,1.42,.18,6),tint,0,y,0);seal.rotation.y=Math.PI/6;}
    add(group,new THREE.BoxGeometry(.28,1.65,.1),tint,0,1.84,1.21);
    const seal=add(group,new THREE.CylinderGeometry(.31,.31,.12,6),GRAPHITE,0,1.82,1.32);seal.rotation.x=Math.PI/2;
  }else if(kind==='entity'){
    for(const x of [-1.35,1.35])add(group,new THREE.BoxGeometry(.24,1.55,1.9),GRAPHITE,x,1,0);
    add(group,new THREE.BoxGeometry(3.6,.25,2.3),PAPER,0,1.83,0);
    add(group,new THREE.BoxGeometry(2.25,2.35,.22),tint,0,2.72,-.88);
    add(group,new THREE.BoxGeometry(1.7,1.8,.1),PAPER,0,2.73,-.72);
    const identity=add(group,new THREE.CylinderGeometry(.31,.31,.08,20),GRAPHITE,0,3.12,-.62);identity.rotation.x=Math.PI/2;
    add(group,new THREE.BoxGeometry(.85,.32,.1),GRAPHITE,0,2.57,-.62);
    add(group,new THREE.BoxGeometry(1.1,.06,.1),GRAPHITE,0,2.21,-.62);
  }else if(kind==='domain_service'){
    for(const x of [-1.75,1.75])for(const z of [-1.2,1.2])add(group,new THREE.CylinderGeometry(.18,.24,2.2,12),PAPER,x,1.35,z);
    add(group,new THREE.BoxGeometry(4.5,.4,3.2),tint,0,2.75+lift,0);
    add(group,new THREE.BoxGeometry(2.5,.18,1.5),PAPER,0,.8,0);
  }else if(kind==='repository'){
    add(group,new THREE.BoxGeometry(4.2,2.25,3.3),PAPER,0,1.45,0);
    for(const x of [-1.2,0,1.2])add(group,new THREE.BoxGeometry(.55,1.65,.16),GRAPHITE,x,1.5,1.72);
    add(group,new THREE.BoxGeometry(4.7,.32,3.7),tint,0,2.82+lift,0);
  }else if(kind==='factory'){
    add(group,new THREE.BoxGeometry(4.2,1.8,3.1),PAPER,0,1.15,0);
    for(const x of [-1.4,0,1.4])roof(group,x,2.33+lift,0,1.25,3.5,tint);
    add(group,new THREE.CylinderGeometry(.3,.36,3.5,12),GRAPHITE,1.65,2,-1.1);
  }else if(kind==='domain_event'){
    add(group,new THREE.CylinderGeometry(1.5,1.5,1.1,8),PAPER,0,.8,0);
    add(group,new THREE.CylinderGeometry(1.8,1.8,.26,8),tint,0,1.58+lift,0);
    add(group,new THREE.BoxGeometry(.14,3.2,.14),GRAPHITE,.65,2.1,0);
    add(group,new THREE.BoxGeometry(1.5,.85,.08),ACCENT,1.33,3.17,0);
  }else if(kind==='specification'){
    add(group,new THREE.BoxGeometry(3.7,1.2,2.9),PAPER,0,.83,0);
    const canopy=add(group,new THREE.BoxGeometry(4.2,.22,3.4),tint,0,2+lift,0);canopy.rotation.z=-.18;
    add(group,new THREE.BoxGeometry(2.3,.12,1.5),GRAPHITE,0,1.6,.25).rotation.x=-.2;
  }else{
    for(const x of [-1.7,1.7])for(const z of [-1.2,1.2])add(group,new THREE.BoxGeometry(.55,.75,.55),STONE,x,.48,z);
    add(group,new THREE.BoxGeometry(3.8,.08,.16),GRAPHITE,0,.9,-1.2);
  }
  if(open){const surface=add(group,new THREE.BoxGeometry(2.7,.13,1.8),PAPER,0,.62,1.7);surface.userData.openedRecord=true;}
  return group;
}
/** A recorded promise has a sentry post; no color here claims code enforcement. */
function guardPost(group:THREE.Group,x:number,z:number,checkpoint=false,scale=1){
  if(checkpoint){
    for(const offset of [-.85,.85])add(group,new THREE.BoxGeometry(.2*scale,1.65*scale,.25*scale),GRAPHITE,x+offset*scale,.9*scale,z);
    add(group,new THREE.BoxGeometry(1.9*scale,.17*scale,.3*scale),ACCENT,x,1.75*scale,z);
    add(group,new THREE.BoxGeometry(1.5*scale,.14*scale,.18*scale),PAPER,x,1.07*scale,z+.16*scale);
  }else{
    add(group,new THREE.BoxGeometry(1.35*scale,.18*scale,1.3*scale),PAPER_EDGE,x,.1*scale,z);
    for(const offset of [-.48,.48])add(group,new THREE.BoxGeometry(.18*scale,1.4*scale,.22*scale),GRAPHITE,x+offset*scale,.86*scale,z);
    add(group,new THREE.CylinderGeometry(.07*scale,.9*scale,.65*scale,4),ACCENT,x,1.76*scale,z);
    const shield=add(group,new THREE.CylinderGeometry(.35*scale,.35*scale,.1*scale,6),PAPER,x,.85*scale,z+.2*scale);shield.rotation.x=Math.PI/2;
  }
}
function protectionMarks(group:THREE.Group,invariants:number,assertions:number){
  if(invariants)guardPost(group,-2.35,2.45,false,.7);
  if(assertions)guardPost(group,1.6,2.5,true,.7);
  group.userData.recordedInvariants=invariants;group.userData.recordedAssertions=assertions;
}
function inspectionCrack(group:THREE.Group){
  const points=[[-1,.28,2.28],[-.55,.42,2.28],[-.28,.2,2.28],[.12,.34,2.28],[.55,.14,2.28],[1,.28,2.28]].map(point=>new THREE.Vector3(...point as [number,number,number]));
  const crack=new THREE.Line(new THREE.BufferGeometry().setFromPoints(points),new THREE.LineBasicMaterial({color:GRAPHITE}));crack.userData.actualFinding=true;group.add(crack);
}
/** Owner-attached inspection evidence. An unsigned notice is not a passing grade. */
function inspectionNotice(group:THREE.Group,x:number,z:number,state:string,scale:number){
  const tone=state==='active'?0xa3633e:state==='baselined'?0xa59772:state==='suppressed'?0x808a95:['coverage','operational','unknown','unavailable'].includes(state)?0x95859e:GRAPHITE;
  add(group,new THREE.BoxGeometry(.12*scale,1.6*scale,.12*scale),GRAPHITE,x,.85*scale,z);
  const board=add(group,new THREE.BoxGeometry(1.3*scale,1.7*scale,.12*scale),PAPER,x,1.35*scale,z);board.userData.inspectionState=state;
  add(group,new THREE.BoxGeometry(.65*scale,.16*scale,.2*scale),tone,x,2.2*scale,z+.02*scale);
  if(['pending','checking','unknown','unavailable'].includes(state)){
    for(const side of [-1,1]){add(group,new THREE.BoxGeometry(.55*scale,.045*scale,.04*scale),tone,x,(1.45+side*.27)*scale,z+.09*scale);add(group,new THREE.BoxGeometry(.045*scale,.55*scale,.04*scale),tone,x+side*.27*scale,1.45*scale,z+.09*scale);}
    if(state==='checking'){const pencil=add(group,new THREE.BoxGeometry(.09*scale,.85*scale,.08*scale),ACCENT,x+.33*scale,1.54*scale,z+.17*scale);pencil.rotation.z=-.55;}
    if(state==='unavailable'){const slash=add(group,new THREE.BoxGeometry(.8*scale,.06*scale,.06*scale),tone,x,1.45*scale,z+.16*scale);slash.rotation.z=.7;}
  }else if(state==='baselined'){
    const tag=add(group,new THREE.BoxGeometry(1.25*scale,.28*scale,.06*scale),tone,x,1.42*scale,z+.1*scale);tag.rotation.z=-.32;
  }else if(state==='suppressed')add(group,new THREE.BoxGeometry(.8*scale,.16*scale,.08*scale),tone,x,1.42*scale,z+.1*scale);
  else if(state==='active'){
    const warning=add(group,new THREE.CylinderGeometry(.42*scale,.42*scale,.06*scale,3),tone,x,1.43*scale,z+.1*scale);warning.rotation.x=Math.PI/2;
  }else add(group,new THREE.BoxGeometry(.8*scale,.055*scale,.06*scale),tone,x,1.42*scale,z+.1*scale);
}
function disposeObject(object: THREE.Object3D) { const geometries = new Set<THREE.BufferGeometry>(), materials = new Set<THREE.Material>(); object.traverse(item => { if (item instanceof CSS2DObject) item.element.remove(); if (item instanceof THREE.Mesh || item instanceof THREE.Line) { if (item.geometry) geometries.add(item.geometry); for (const material of Array.isArray(item.material) ? item.material : [item.material]) materials.add(material); } }); geometries.forEach(geometry => geometry.dispose()); materials.forEach(material => { (material as THREE.MeshBasicMaterial).map?.dispose(); material.dispose(); }); }
function fallback(host: HTMLElement, callbacks: SceneCallbacks): SceneController {
  host.classList.add('study-fallback');
  function update(state: SceneState) {
    const active=document.activeElement,focusedId=active instanceof HTMLElement && host.contains(active)?active.dataset.subjectId:undefined;
    const view = state.view ?? projectView(state.project, { scopeId: state.scopeId, selectedId: state.selectedId }); host.replaceChildren(); host.dataset.viewLevel = view.level;
    const note = document.createElement('p'); note.textContent = '3D is unavailable. The same focused subjects remain available below.'; host.append(note);
    for (const id of view.ids.slice(0, 8)) { const subject = [...state.project.contexts, ...state.project.concepts].find(item => item.id === id); if (!subject) continue; const button = document.createElement('button'); button.className = 'atlas-label'; button.dataset.subjectId = id; button.textContent = subject.name; button.addEventListener('click', () => 'kind' in subject ? callbacks.select(id) : callbacks.enter?.(id)); host.append(button); }
    if(focusedId && (!document.activeElement || document.activeElement===document.body))Array.from(host.querySelectorAll<HTMLButtonElement>('[data-subject-id]')).find(button=>button.dataset.subjectId===focusedId)?.focus({preventScroll:true});
  }
  return { update, focus(id) { Array.from(host.querySelectorAll<HTMLButtonElement>('[data-subject-id]')).find(item => item.dataset.subjectId === id)?.focus(); }, overview() { host.scrollTop = 0; }, topView() { host.scrollTop = 0; }, zoom() {}, dispose() { host.remove(); }, screenPosition() { return null; } };
}
