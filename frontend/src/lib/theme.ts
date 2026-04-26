import { writable } from 'svelte/store';

export type Theme = 'dark' | 'light';

const KEY = 'rungine.theme';

function read(): Theme {
  const v = localStorage.getItem(KEY);
  return v === 'light' ? 'light' : 'dark';
}

export const theme = writable<Theme>(read());

export function setTheme(t: Theme) {
  document.documentElement.dataset.theme = t;
  localStorage.setItem(KEY, t);
  theme.set(t);
}

export function initTheme() {
  setTheme(read());
}
