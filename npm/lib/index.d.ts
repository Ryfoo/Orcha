// Type declarations for orcha-dev. Hand-written rather than generated so the
// runtime stays plain JS with no build step.

export type OutputType = 'text' | 'json' | 'filepath' | 'list';

export type EventType =
  | 'task_start'
  | 'task_complete'
  | 'task_fail'
  | 'pipeline_complete';

/** One streaming event from a pipeline run. Mirrors the Python OrchaEvent. */
export interface OrchaEvent {
  type: EventType;
  /** Name of the task; empty string on `pipeline_complete` in v1. */
  task: string;
  /** 0-based position in the pipeline; -1 for parse-time errors. */
  index: number;
  /** Final/intermediate output. `null` for `task_start` and `task_fail`. */
  output: unknown;
  /** Output type tag from the YAML; empty for `task_start`/`task_fail`. */
  output_type: OutputType | '';
  /** Error message; only set when `type === 'task_fail'`. */
  error?: string | null;
  /** Wall-clock duration of this step in ms. 0 for `task_start`. */
  elapsed_ms: number;
}

export interface CreateOptions {
  /** Absolute path to a pre-built engine binary (skips download). */
  binaryPath?: string;
}

export type RunInput = string | Buffer | Uint8Array | object | unknown[] | null | undefined;

export class Orcha {
  /** Use {@link Orcha.create} instead unless you already have a binary path. */
  constructor(yamlPath: string, binaryPath: string);

  /** Resolve (and lazily download + verify) the engine binary, then return an Orcha bound to a YAML file. */
  static create(yamlPath: string, options?: CreateOptions): Promise<Orcha>;

  /** Absolute path to the resolved orcha.yaml. */
  readonly yamlPath: string;
  /** Absolute path to the engine binary in use. */
  readonly binary: string;

  /** Run a pipeline and stream events as they arrive. */
  run(target: string, input?: RunInput): AsyncIterable<OrchaEvent>;

  /**
   * Run a pipeline to completion and return its final output. Throws
   * {@link TaskFailed} on the first failing task. Equivalent to Python's
   * `run_sync`.
   */
  runToCompletion(target: string, input?: RunInput): Promise<unknown>;
}

export class OrchaError extends Error {
  readonly name: 'OrchaError' | 'TaskFailed';
}

export class TaskFailed extends OrchaError {
  readonly name: 'TaskFailed';
  readonly task: string;
  readonly index: number;
  readonly taskError: string;
}
