'use strict';

// OrchaError covers anything that goes wrong outside a task — binary
// download/verification, IPC plumbing, missing yaml, etc. Task-level failures
// surface as TaskFailed.
class OrchaError extends Error {
  constructor(message) {
    super(message);
    this.name = 'OrchaError';
  }
}

// TaskFailed is thrown by runToCompletion() when the engine emits a
// task_fail event, so callers can use plain try/catch.
class TaskFailed extends OrchaError {
  constructor(task, index, error) {
    super(`task ${task} (index ${index}) failed: ${error}`);
    this.name = 'TaskFailed';
    this.task = task;
    this.index = index;
    this.taskError = error;
  }
}

module.exports = { OrchaError, TaskFailed };
