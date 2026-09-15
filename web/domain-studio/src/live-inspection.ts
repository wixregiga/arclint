import { queryRepositoryRevision, type RepositoryRevision } from './repository';

interface Options {
  ready?: Promise<unknown>;
  check(isCurrent: () => boolean): Promise<void>;
  watchState(note: string): void;
  revision?(signal: AbortSignal): Promise<RepositoryRevision>;
  /** Short intervals are injectable for lifecycle tests. */
  pollMs?: number; settleMs?: number;
}

/** Repository changes refresh evidence without navigating or rewriting a draft. */
export function startLiveInspection(options: Options) {
  const pollMs = options.pollMs ?? 2500, settleMs = options.settleMs ?? 650;
  const revision = options.revision ?? queryRepositoryRevision;
  const abort = new AbortController();
  let stopped = false, running = false, polling = false, pending = false, initialized = false, ready = false;
  let token: string | undefined, generation = 0, visibilityGeneration = 0, failures = 0, lastFallback = 0;
  let lastNote: string | undefined;
  let pollTimer: ReturnType<typeof setTimeout> | undefined;
  let checkTimer: ReturnType<typeof setTimeout> | undefined;
  const visible = () => typeof document === 'undefined' || document.visibilityState !== 'hidden';
  function watchState(note: string) { if (lastNote !== note) { lastNote = note; options.watchState(note); } }
  function queue(delay = settleMs) {
    pending = true;
    clearTimeout(checkTimer);
    checkTimer = undefined;
    if (stopped || !ready || !visible()) return;
    checkTimer = setTimeout(() => { checkTimer = undefined; void inspect(); }, delay);
  }
  async function inspect() {
    if (stopped || running || !pending || !visible()) return;
    running = true; pending = false;
    const started = generation;
    try {
      await options.check(() => !stopped && visible() && started === generation);
      failures = 0;
    } catch {
      // The evaluator exposes the error in the scene. Retry quietly with backoff.
      failures++;
      pending = true;
    } finally {
      running = false;
      if (!stopped && pending) queue(failures ? Math.min(30000, 1500 * 2 ** Math.min(failures, 5)) : settleMs);
    }
  }
  async function poll() {
    if (stopped || polling || !ready) return;
    polling = true;
    const visibilityAtStart = visibilityGeneration;
    if (visible()) {
      try {
        const current = await revision(abort.signal);
        if (stopped) { polling = false; return; }
        if (visibilityAtStart !== visibilityGeneration) {
          polling = false;
          if (visible()) void poll();
          return;
        }
        if (typeof current.revision !== 'string' || typeof current.watching !== 'boolean') throw new Error('Revision unavailable');
        watchState(current.watching ? '' : current.reason ?? 'Live updates unavailable; checking periodically.');
        if (!initialized || token !== current.revision) {
          token = current.revision; generation++; queue();
          if (!current.watching) lastFallback = Date.now();
        } else if (!current.watching && Date.now() - lastFallback > 60000) {
          generation++; queue(); lastFallback = Date.now();
        }
      } catch {
        if (stopped) { polling = false; return; }
        watchState('Live updates unavailable; checking periodically.');
        if (!initialized || Date.now() - lastFallback > 60000) { generation++; queue(); lastFallback = Date.now(); }
      }
      initialized = true;
      if (pending && !running && checkTimer === undefined) queue();
    }
    polling = false;
    if (!stopped && visible()) pollTimer = setTimeout(() => void poll(), pollMs);
  }
  function resume() {
    if (stopped) return;
    clearTimeout(pollTimer);
    if (!visible()) {
      visibilityGeneration++;
      clearTimeout(checkTimer); checkTimer = undefined;
      // While hidden there are no revision samples, so an unfinished result
      // cannot establish freshness. Poll again before resuming its replacement.
      if (running) { generation++; pending = true; }
      return;
    }
    void poll();
  }
  void (options.ready ?? Promise.resolve()).catch(() => {}).then(() => {
    ready = true;
    if (!stopped) void poll();
  });
  if (typeof document !== 'undefined') document.addEventListener('visibilitychange', resume);
  return { invalidate() { if (!stopped) { generation++; queue(); } }, dispose() {
    stopped = true; generation++; abort.abort(); clearTimeout(pollTimer); clearTimeout(checkTimer);
    if (typeof document !== 'undefined') document.removeEventListener('visibilitychange', resume);
  } };
}
