import { act, getState } from "../store";

export const transport = {
  toggle: () => act((api) => (getState().clock?.playing ? api.Pause() : api.Play())),
  step: () => act((api) => api.StepForward()),
  back: () => act((api) => api.StepBack()),
  reset: () => act((api) => api.Reset()),
  drop: () => act((api) => api.DropNext(getState().dropFrom)),
  home: () => act((api) => api.SeekTime(0)),
  end: () => act((api) => api.SeekTime(getState().clock?.end ?? 0)),
};
