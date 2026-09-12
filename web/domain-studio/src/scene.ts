import * as THREE from 'three';
import { OrbitControls } from 'three/addons/controls/OrbitControls.js';
import { CSS2DObject, CSS2DRenderer } from 'three/addons/renderers/CSS2DRenderer.js';
import type { Concept, DomainContext, SceneCallbacks, SceneController, SceneState } from './contracts';
import './scene.css';

const ION = 0x66d8df;
const AMBER = 0xf6b85c;

/** Positions are world coordinates. Moving a shape never changes domain membership. */
export function createScene(container: HTMLElement, callbacks: SceneCallbacks): SceneController {
  const host = document.createElement('div');
  host.className = 'domain-scene';
  container.append(host);
  let renderer: THREE.WebGLRenderer;
  try {
    renderer = new THREE.WebGLRenderer({ antialias: true, alpha: true, powerPreference: 'high-performance' });
  } catch {
    return fallbackScene(host, callbacks);
  }
  renderer.setPixelRatio(Math.min(window.devicePixelRatio || 1, 2));
  renderer.outputColorSpace = THREE.SRGBColorSpace;
  renderer.setClearColor(0x080f18, 1);
  renderer.domElement.setAttribute('aria-label', 'Spatial domain map. Drag to orbit, right-drag to pan, scroll to zoom. Select named objects to edit. Shift-drag a concept to arrange it.');
  renderer.domElement.tabIndex = 0;
  host.append(renderer.domElement);
  const labels = new CSS2DRenderer();
  labels.domElement.className = 'scene-labels';
  host.append(labels.domElement);

  const scene = new THREE.Scene();
  scene.fog = new THREE.FogExp2(0x080f18, .0018);
  const camera = new THREE.PerspectiveCamera(43, 1, .1, 2000);
  camera.position.set(65, 74, 105);
  const controls = new OrbitControls(camera, renderer.domElement);
  controls.enableDamping = !window.matchMedia('(prefers-reduced-motion: reduce)').matches;
  controls.dampingFactor = .08;
  controls.minDistance = 8;
  controls.maxDistance = 500;
  controls.maxPolarAngle = Math.PI * .49;
  controls.target.set(0, 0, 0);
  scene.add(new THREE.AmbientLight(0x8cbdcd, 2.1));
  const key = new THREE.DirectionalLight(0x9bcfff, 4.3);
  key.position.set(20, 70, 30);
  scene.add(key);
  const rim = new THREE.DirectionalLight(AMBER, 1.7);
  rim.position.set(-50, 25, -40);
  scene.add(rim);
  const backdrop = buildBackdrop();
  scene.add(backdrop);
  let world = new THREE.Group();
  scene.add(world);
  let state: SceneState | null = null;
  let initialized = false;
  let disposed = false;
  let frame = 0;
  const targets: THREE.Object3D[] = [];
  const locations = new Map<string, THREE.Vector3>();
  const groups = new Map<string, THREE.Group>();
  const raycaster = new THREE.Raycaster();
  const pointer = new THREE.Vector2();
  let pressed: { x: number; y: number } | null = null;
  let drag: { id: string; plane: THREE.Plane; origin: THREE.Vector3; offset: THREE.Vector3; moved: boolean } | null = null;

  const resize = () => {
    const width = Math.max(host.clientWidth, 1);
    const height = Math.max(host.clientHeight, 1);
    renderer.setSize(width, height);
    labels.setSize(width, height);
    camera.aspect = width / height;
    camera.updateProjectionMatrix();
  };
  const observer = new ResizeObserver(resize);
  observer.observe(host);
  resize();

  function screenRay(event: PointerEvent) {
    const bounds = renderer.domElement.getBoundingClientRect();
    pointer.set((event.clientX - bounds.left) / bounds.width * 2 - 1, -(event.clientY - bounds.top) / bounds.height * 2 + 1);
    raycaster.setFromCamera(pointer, camera);
  }

  function pick(event: PointerEvent): string | null {
    screenRay(event);
    return raycaster.intersectObjects(targets, true)[0]?.object.userData.subjectId ?? null;
  }

  function beginDrag(event: PointerEvent, id: string) {
    const concept = state?.project.concepts.find(item => item.id === id);
    if (!concept || event.button !== 0) return;
    event.preventDefault();
    event.stopPropagation();
    screenRay(event);
    const origin = new THREE.Vector3(...concept.position);
    const plane = new THREE.Plane(new THREE.Vector3(0, 1, 0), -origin.y);
    const hit = raycaster.ray.intersectPlane(plane, new THREE.Vector3());
    if (!hit) return;
    drag = { id, plane, origin, offset: origin.clone().sub(hit), moved: false };
    controls.enabled = false;
    host.classList.add('is-dragging');
  }

  function onPointerDown(event: PointerEvent) {
    if (event.button !== 0) return;
    pressed = { x: event.clientX, y: event.clientY };
    if (event.shiftKey) {
      const id = pick(event);
      if (id) beginDrag(event, id);
    }
  }

  function onPointerMove(event: PointerEvent) {
    if (!drag) return;
    screenRay(event);
    const hit = raycaster.ray.intersectPlane(drag.plane, new THREE.Vector3());
    if (!hit) return;
    hit.add(drag.offset);
    const group = groups.get(drag.id);
    if (group) group.position.copy(hit);
    drag.moved = hit.distanceTo(drag.origin) > .05;
  }

  function onPointerUp(event: PointerEvent) {
    if (drag) {
      const position = groups.get(drag.id)?.position;
      if (position && drag.moved) callbacks.move(drag.id, [round(position.x), round(position.y), round(position.z)]);
      drag = null;
      controls.enabled = true;
      host.classList.remove('is-dragging');
      pressed = null;
      return;
    }
    if (pressed && Math.hypot(event.clientX - pressed.x, event.clientY - pressed.y) < 5 && event.target === renderer.domElement) {
      callbacks.select(pick(event));
    }
    pressed = null;
  }

  function onKeyDown(event: KeyboardEvent) {
    if (event.key === 'Escape') callbacks.select(null);
    if (event.key === 'Home') { event.preventDefault(); overview(); }
    if (event.key === '+' || event.key === '=') zoom(1);
    if (event.key === '-') zoom(-1);
  }

  renderer.domElement.addEventListener('pointerdown', onPointerDown, true);
  renderer.domElement.addEventListener('keydown', onKeyDown);
  window.addEventListener('pointermove', onPointerMove);
  window.addEventListener('pointerup', onPointerUp);
  window.addEventListener('pointercancel', onPointerUp);

  function makeLabel(id: string, text: string, type: 'concept' | 'context' | 'relationship', color: string, kind?: string, ghost = false) {
    const button = document.createElement('button');
    button.className = `scene-label scene-label--${type}${ghost ? ' scene-label--ghost' : ''}`;
    button.style.setProperty('--node-color', color);
    button.dataset.subjectId = id;
    button.title = text;
    button.setAttribute('aria-pressed', String(state?.selectedId === id));
    button.setAttribute('aria-label', `${ghost ? 'Baseline: ' : ''}${text}${kind ? `, ${kind.replaceAll('_', ' ')}` : ''}`);
    const strong = document.createElement('strong');
    strong.textContent = text;
    button.append(strong);
    if (kind) {
      const small = document.createElement('small');
      small.textContent = kind.replaceAll('_', ' ');
      button.append(small);
    }
    if (state?.findings.some(finding => finding.subjectId === id)) {
      const issue = document.createElement('span');
      issue.className = 'scene-issue';
      issue.textContent = '●';
      issue.setAttribute('aria-label', 'has model issues');
      strong.append(issue);
    }
    if (state?.search && !text.toLowerCase().includes(state.search.toLowerCase())) button.classList.add('scene-label--dim');
    if (ghost) {
      button.disabled = true;
      button.tabIndex = -1;
    } else {
      button.addEventListener('click', event => { event.stopPropagation(); if (!event.shiftKey) callbacks.select(id); });
      button.addEventListener('dblclick', () => focus(id));
      button.addEventListener('pointerdown', event => { if (event.shiftKey) beginDrag(event, id); });
    }
    return new CSS2DObject(button);
  }

  function buildContext(context: DomainContext, ghost = false) {
    const group = new THREE.Group();
    group.position.set(...context.position);
    const members = (ghost && state!.baseline ? state!.baseline.project : state!.project).concepts.filter(concept => concept.contextId === context.id);
    const radius = Math.max(12, ...members.map(concept => Math.hypot(concept.position[0] - context.position[0], concept.position[2] - context.position[2]) + 5));
    const color = ghost ? new THREE.Color(0xef9377) : safeColor(context.color);
    const material = new THREE.MeshBasicMaterial({ color, transparent: true, opacity: ghost ? .015 : .035, side: THREE.DoubleSide, depthWrite: false });
    const disk = new THREE.Mesh(new THREE.CircleGeometry(radius, 6), material);
    disk.rotation.x = -Math.PI / 2;
    disk.position.y = -.7;
    disk.userData.subjectId = context.id;
    group.add(disk);
    if (!ghost) targets.push(disk);
    const ring = new THREE.LineLoop(new THREE.BufferGeometry().setFromPoints(Array.from({ length: 6 }, (_, i) => new THREE.Vector3(Math.cos(i * Math.PI / 3) * radius, -.65, Math.sin(i * Math.PI / 3) * radius))), new THREE.LineBasicMaterial({ color, transparent: true, opacity: ghost ? .14 : .35 }));
    group.add(ring);
    const inner = new THREE.LineLoop(new THREE.BufferGeometry().setFromPoints(Array.from({ length: 6 }, (_, i) => new THREE.Vector3(Math.cos(i * Math.PI / 3) * (radius - .35), -.64, Math.sin(i * Math.PI / 3) * (radius - .35)))), new THREE.LineBasicMaterial({ color, transparent: true, opacity: .1 }));
    group.add(inner);
    // Small angular corner beacons make territories legible without filling the map.
    for (let i = 0; i < 6; i++) {
      const marker = new THREE.Mesh(new THREE.BoxGeometry(.13, .18, 1.3), new THREE.MeshBasicMaterial({ color, transparent: true, opacity: .65 }));
      marker.position.set(Math.cos(i * Math.PI / 3) * radius, -.4, Math.sin(i * Math.PI / 3) * radius);
      marker.rotation.y = -i * Math.PI / 3;
      group.add(marker);
    }
    const label = makeLabel(context.id, ghost ? `Previous · ${context.name}` : context.name, 'context', `#${color.getHexString()}`, undefined, ghost);
    label.position.set(0, .1, radius + (ghost ? 6 : 3));
    group.add(label);
    if (state?.selectedId === context.id && !ghost) material.opacity = .075;
    world.add(group);
    if (!ghost || !locations.has(context.id)) locations.set(context.id, group.position.clone());
  }

  function buildConcept(concept: Concept, ghost = false) {
    const group = new THREE.Group();
    group.position.set(...concept.position);
    const color = ghost ? new THREE.Color(0xef9377) : safeColor(state!.project.contexts.find(context => context.id === concept.contextId)?.color ?? '#66d8df');
    const selected = !ghost && state!.selectedId === concept.id;
    const dim = !!state!.search && !concept.name.toLowerCase().includes(state!.search.toLowerCase());
    const geometry = conceptGeometry(concept.kind);
    const material = new THREE.MeshStandardMaterial({ color: ghost ? 0x553c3b : 0x385362, roughness: .62, metalness: .32, emissive: color, emissiveIntensity: selected ? .3 : .085, transparent: ghost || dim, opacity: ghost ? .1 : dim ? .2 : 1, wireframe: ghost });
    const body = new THREE.Mesh(geometry, material);
    body.position.y = 1.6;
    body.userData.subjectId = concept.id;
    group.add(body);
    const edges = new THREE.LineSegments(new THREE.EdgesGeometry(geometry), new THREE.LineBasicMaterial({ color, transparent: true, opacity: ghost ? .28 : dim ? .15 : selected ? 1 : .65 }));
    edges.position.copy(body.position);
    group.add(edges);
    if (!ghost) targets.push(body);
    // An offset shoulder and illuminated spine create a deliberate, sculptural silhouette.
    const shoulder = new THREE.Mesh(new THREE.CylinderGeometry(.72, 1.05, 2.6, 4), material);
    shoulder.position.set(-1.05, .85, .7);
    shoulder.rotation.y = Math.PI / 4;
    shoulder.userData.subjectId = concept.id;
    group.add(shoulder);
    if (!ghost) targets.push(shoulder);
    const core = new THREE.Mesh(new THREE.BoxGeometry(.16, 2.1, .2), new THREE.MeshBasicMaterial({ color, transparent: true, opacity: ghost ? .15 : dim ? .2 : .95 }));
    core.position.set(-.95, 1.1, 1.38);
    group.add(core);
    const foot = new THREE.Mesh(new THREE.CylinderGeometry(2.4, 2.9, .32, 6), new THREE.MeshStandardMaterial({ color: 0x112734, roughness: .8, metalness: .6, transparent: ghost || dim, opacity: ghost ? .12 : dim ? .2 : 1 }));
    foot.position.y = -.3;
    foot.userData.subjectId = concept.id;
    group.add(foot);
    if (!ghost) targets.push(foot);
    const ring = new THREE.Mesh(new THREE.RingGeometry(selected ? 3 : 2.6, selected ? 3.12 : 2.65, 6), new THREE.MeshBasicMaterial({ color, side: THREE.DoubleSide, transparent: true, opacity: ghost ? .2 : dim ? .12 : selected ? 1 : .55, depthWrite: false }));
    ring.rotation.x = -Math.PI / 2;
    ring.position.y = -.08;
    group.add(ring);
    if (selected) {
      const beam = new THREE.Mesh(new THREE.CylinderGeometry(.035, .035, 7, 4), new THREE.MeshBasicMaterial({ color: ION, transparent: true, opacity: .35 }));
      beam.position.y = 5;
      group.add(beam);
      const mark = new THREE.Mesh(new THREE.OctahedronGeometry(.27), new THREE.MeshBasicMaterial({ color: ION }));
      mark.position.y = 8.5;
      group.add(mark);
    }
    const label = makeLabel(concept.id, concept.name, 'concept', `#${color.getHexString()}`, ghost ? 'previous state' : concept.kind, ghost);
    label.position.set(0, .3, 1.9);
    group.add(label);
    world.add(group);
    if (!ghost) {
      groups.set(concept.id, group);
      locations.set(concept.id, group.position.clone());
    } else if (!locations.has(concept.id)) locations.set(concept.id, group.position.clone());
  }

  function update(next: SceneState) {
    if (disposed) return;
    state = next;
    scene.remove(world);
    disposeObject(world);
    world = new THREE.Group();
    scene.add(world);
    targets.length = 0;
    groups.clear();
    locations.clear();
    next.project.contexts.forEach(context => buildContext(context));
    next.project.concepts.forEach(concept => buildConcept(concept));
    const visibleRelationships = next.project.relationships.map(relationship => ({ ...relationship, derived: false }));
    for (const concept of next.project.concepts) {
      if (!concept.ownerId || !['entity', 'repository', 'factory'].includes(concept.kind)) continue;
      if (!next.project.concepts.some(owner => owner.id === concept.ownerId && owner.kind === 'aggregate')) continue;
      const source = concept.kind === 'entity' ? concept.ownerId : concept.id;
      const target = concept.kind === 'entity' ? concept.id : concept.ownerId;
      const explicitlyConnected = next.project.relationships.some(relationship =>
        (relationship.source === source && relationship.target === target)
        || (relationship.source === target && relationship.target === source));
      if (explicitlyConnected) continue;
      visibleRelationships.push({
        id: concept.id,
        source,
        target,
        label: concept.kind === 'entity' ? 'owns member' : concept.kind === 'repository' ? 'repository for' : 'creates',
        derived: true,
      });
    }
    for (const relationship of visibleRelationships) {
      const from = locations.get(relationship.source);
      const to = locations.get(relationship.target);
      if (!from || !to) continue;
      const start = from.clone().add(new THREE.Vector3(0, 1.8, 0));
      const end = to.clone().add(new THREE.Vector3(0, 1.8, 0));
      const middle = start.clone().lerp(end, .5);
      middle.y += Math.min(10, Math.max(3, start.distanceTo(end) * .14));
      const curve = new THREE.QuadraticBezierCurve3(start, middle, end);
      const selected = next.selectedId === relationship.id || next.selectedId === relationship.source || next.selectedId === relationship.target;
      const line = new THREE.Line(new THREE.BufferGeometry().setFromPoints(curve.getPoints(48)), new THREE.LineBasicMaterial({ color: selected ? ION : 0x688d9b, transparent: true, opacity: selected ? .95 : relationship.derived ? .28 : .44 }));
      world.add(line);
      const point = curve.getPoint(.8);
      const tangent = curve.getTangent(.8).normalize();
      const arrow = new THREE.Mesh(new THREE.ConeGeometry(.24, .8, 4), new THREE.MeshBasicMaterial({ color: selected ? ION : 0x8ba7b0, transparent: true, opacity: .8 }));
      arrow.position.copy(point);
      arrow.quaternion.setFromUnitVectors(new THREE.Vector3(0, 1, 0), tangent);
      arrow.userData.subjectId = relationship.id;
      world.add(arrow);
      targets.push(arrow);
      const label = makeLabel(relationship.id, relationship.label || 'relationship', 'relationship', '#91a6b5');
      label.element.classList.toggle('is-revealed', selected);
      label.position.copy(curve.getPoint(.5));
      world.add(label);
      if (!relationship.derived) locations.set(relationship.id, label.position.clone());
    }
    if (next.mode === 'baseline' && next.baseline) {
      for (const prior of next.baseline.project.concepts) {
        const current = next.project.concepts.find(item => item.id === prior.id);
        if (!current || JSON.stringify(current) !== JSON.stringify(prior)) {
          const shifted = { ...prior, position: [...prior.position] as [number, number, number] };
          if (current && current.position.every((value, index) => value === prior.position[index])) shifted.position[1] += 4.5;
          buildConcept(shifted, true);
        }
      }
      for (const prior of next.baseline.project.contexts) {
        const current = next.project.contexts.find(item => item.id === prior.id);
        if (!current || JSON.stringify(current) !== JSON.stringify(prior)) buildContext(prior, true);
      }
      for (const prior of next.baseline.project.relationships) {
        const current = next.project.relationships.find(item => item.id === prior.id);
        if (current && JSON.stringify(current) === JSON.stringify(prior)) continue;
        const baselineSubjects = [...next.baseline.project.concepts, ...next.baseline.project.contexts];
        const from = baselineSubjects.find(item => item.id === prior.source);
        const to = baselineSubjects.find(item => item.id === prior.target);
        if (!from || !to) continue;
        const start = new THREE.Vector3(...from.position).add(new THREE.Vector3(0, 3.5, 0));
        const end = new THREE.Vector3(...to.position).add(new THREE.Vector3(0, 3.5, 0));
        const midpoint = start.clone().lerp(end, .5).add(new THREE.Vector3(0, 5, 0));
        const line = new THREE.Line(new THREE.BufferGeometry().setFromPoints(new THREE.QuadraticBezierCurve3(start, midpoint, end).getPoints(36)), new THREE.LineDashedMaterial({ color: 0xef9377, transparent: true, opacity: .45, dashSize: .6, gapSize: .4 }));
        line.computeLineDistances();
        world.add(line);
        if (!locations.has(prior.id)) locations.set(prior.id, midpoint.clone());
      }
    }
    if (!initialized && (next.project.contexts.length || next.project.concepts.length)) {
      initialized = true;
      overview();
    }
  }

  function bounds() {
    const positions = [...locations.values()];
    if (!positions.length) return { center: new THREE.Vector3(), size: 60 };
    const box = new THREE.Box3().setFromPoints(positions);
    const extent = box.getSize(new THREE.Vector3());
    return { center: box.getCenter(new THREE.Vector3()), size: Math.max(45, extent.x + 25, extent.z + 25) };
  }

  function overview() {
    const { center, size } = bounds();
    const distance = size * (.94 / Math.min(camera.aspect, 1.35));
    controls.target.copy(center);
    camera.position.copy(center).add(new THREE.Vector3(distance * .42, distance * .82, distance));
    controls.update();
  }

  function focus(id: string) {
    const point = locations.get(id);
    if (!point) return;
    const context = state?.project.contexts.some(item => item.id === id)
      || (state?.mode === 'baseline' && state.baseline?.project.contexts.some(item => item.id === id));
    const distance = context ? 50 : 22;
    const direction = camera.position.clone().sub(controls.target).normalize();
    controls.target.copy(point);
    camera.position.copy(point).addScaledVector(direction, distance);
    controls.update();
  }

  function topView() {
    const { center, size } = bounds();
    controls.target.copy(center);
    // Fit the shorter screen dimension, leaving a small breathing room around territories.
    const distance = size / (2 * Math.tan(THREE.MathUtils.degToRad(camera.fov) / 2) * Math.min(camera.aspect, 1)) * 1.06;
    camera.position.copy(center).add(new THREE.Vector3(0, distance, .01));
    controls.update();
  }

  function zoom(direction: number) {
    const offset = camera.position.clone().sub(controls.target);
    offset.setLength(THREE.MathUtils.clamp(offset.length() * (direction > 0 ? .8 : 1.25), controls.minDistance, controls.maxDistance));
    camera.position.copy(controls.target).add(offset);
    controls.update();
  }

  function animate() {
    if (disposed) return;
    frame = requestAnimationFrame(animate);
    controls.update();
    renderer.render(scene, camera);
    labels.render(scene, camera);
  }
  animate();

  return {
    update, focus, overview, topView, zoom,
    dispose() {
      disposed = true;
      cancelAnimationFrame(frame);
      observer.disconnect();
      controls.dispose();
      renderer.domElement.removeEventListener('pointerdown', onPointerDown, true);
      renderer.domElement.removeEventListener('keydown', onKeyDown);
      window.removeEventListener('pointermove', onPointerMove);
      window.removeEventListener('pointerup', onPointerUp);
      window.removeEventListener('pointercancel', onPointerUp);
      disposeObject(scene);
      renderer.dispose();
      host.remove();
    },
  };
}

function conceptGeometry(kind: Concept['kind']): THREE.BufferGeometry {
  switch (kind) {
    case 'aggregate': return new THREE.CylinderGeometry(1.4, 2.1, 3.8, 5);
    case 'entity': return new THREE.BoxGeometry(2.2, 2.8, 2.2);
    case 'value_object': return new THREE.OctahedronGeometry(1.9);
    case 'domain_event': return new THREE.ConeGeometry(1.7, 3.3, 3);
    case 'domain_service': return new THREE.CylinderGeometry(1.7, 1.7, 2.2, 6);
    case 'repository': return new THREE.BoxGeometry(2.9, 1.8, 1.8);
    case 'factory': return new THREE.ConeGeometry(1.8, 3.5, 4);
    case 'specification': return new THREE.IcosahedronGeometry(1.8, 0);
    default: return new THREE.CylinderGeometry(.75, 1.55, 3.8, 3);
  }
}

function safeColor(value: string): THREE.Color {
  return /^#[\da-f]{6}$/i.test(value) ? new THREE.Color(value) : new THREE.Color(ION);
}

function round(value: number) { return Math.round(value * 100) / 100; }

function disposeObject(object: THREE.Object3D) {
  const textures = new Set<THREE.Texture>();
  const materials = new Set<THREE.Material>();
  const geometries = new Set<THREE.BufferGeometry>();
  object.traverse(item => {
    if (item instanceof CSS2DObject) item.element.remove();
    const drawable = item as THREE.Mesh;
    if (drawable.geometry) geometries.add(drawable.geometry);
    if (drawable.material) for (const material of Array.isArray(drawable.material) ? drawable.material : [drawable.material]) materials.add(material);
  });
  materials.forEach(material => {
    const map = (material as THREE.MeshBasicMaterial).map;
    if (map) textures.add(map);
    material.dispose();
  });
  textures.forEach(texture => texture.dispose());
  geometries.forEach(geometry => geometry.dispose());
}

function buildBackdrop(): THREE.Group {
  const group = new THREE.Group();
  let seed = 8217;
  const random = () => { seed = (seed * 16807) % 2147483647; return (seed - 1) / 2147483646; };
  const positions: number[] = [];
  for (let index = 0; index < 1800; index++) {
    const azimuth = random() * Math.PI * 2;
    const elevation = Math.acos(random() * 2 - 1);
    const radius = 380 + random() * 470;
    positions.push(radius * Math.sin(elevation) * Math.cos(azimuth), radius * Math.cos(elevation), radius * Math.sin(elevation) * Math.sin(azimuth));
  }
  const starGeometry = new THREE.BufferGeometry();
  starGeometry.setAttribute('position', new THREE.Float32BufferAttribute(positions, 3));
  group.add(new THREE.Points(starGeometry, new THREE.PointsMaterial({ color: 0xa3beca, size: .65, transparent: true, opacity: .48, sizeAttenuation: true, depthWrite: false, fog: false })));
  const grid = new THREE.GridHelper(600, 100, 0x213b4b, 0x142736);
  grid.position.y = -2;
  (grid.material as THREE.Material).transparent = true;
  (grid.material as THREE.Material).opacity = .23;
  group.add(grid);
  // A generated radial haze gives atmospheric depth without external assets.
  const canvas = document.createElement('canvas');
  canvas.width = canvas.height = 256;
  const context = canvas.getContext('2d');
  if (context) {
    const gradient = context.createRadialGradient(128, 128, 0, 128, 128, 128);
    gradient.addColorStop(0, 'rgba(58,108,124,0.22)');
    gradient.addColorStop(.4, 'rgba(30,64,88,0.11)');
    gradient.addColorStop(1, 'rgba(8,15,24,0)');
    context.fillStyle = gradient;
    context.fillRect(0, 0, 256, 256);
    const haze = new THREE.Sprite(new THREE.SpriteMaterial({ map: new THREE.CanvasTexture(canvas), transparent: true, depthWrite: false, fog: false, blending: THREE.AdditiveBlending }));
    haze.position.set(-90, 25, -130);
    haze.scale.set(440, 240, 1);
    group.add(haze);
  }
  return group;
}

function fallbackScene(host: HTMLElement, callbacks: SceneCallbacks): SceneController {
  host.classList.add('scene-fallback');
  function update(state: SceneState) {
    host.replaceChildren();
    const notice = document.createElement('p');
    notice.className = 'scene-webgl-notice';
    notice.textContent = '3D rendering is unavailable in this browser. Your domain remains editable through this accessible model view.';
    host.append(notice);
    const addButton = (parent: HTMLElement, id: string, name: string, detail: string) => {
      const button = document.createElement('button');
      button.dataset.subjectId = id;
      button.setAttribute('aria-pressed', String(state.selectedId === id));
      button.textContent = name;
      const small = document.createElement('small');
      small.textContent = detail;
      button.append(small);
      button.addEventListener('click', () => callbacks.select(id));
      parent.append(button);
    };
    for (const context of state.project.contexts) {
      const section = document.createElement('section');
      addButton(section, context.id, context.name, 'Bounded context');
      for (const concept of state.project.concepts.filter(item => item.contextId === context.id && (!state.search || item.name.toLowerCase().includes(state.search.toLowerCase())))) {
        addButton(section, concept.id, concept.name, concept.kind.replaceAll('_', ' '));
      }
      host.append(section);
    }
    for (const concept of state.project.concepts.filter(item => !state.project.contexts.some(context => context.id === item.contextId))) addButton(host, concept.id, concept.name, concept.kind.replaceAll('_', ' '));
    for (const relationship of state.project.relationships) {
      const from = state.project.concepts.find(item => item.id === relationship.source)?.name ?? relationship.source;
      const to = state.project.concepts.find(item => item.id === relationship.target)?.name ?? relationship.target;
      addButton(host, relationship.id, `${from} → ${to}`, relationship.label);
    }
  }
  return {
    update,
    focus(id) { Array.from(host.querySelectorAll<HTMLButtonElement>('button')).find(button => button.dataset.subjectId === id)?.focus(); },
    overview() { host.scrollTop = 0; },
    topView() { host.scrollTop = 0; },
    zoom() {},
    dispose() { host.remove(); },
  };
}
