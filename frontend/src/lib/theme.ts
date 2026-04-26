import { writable } from 'svelte/store';

export type Theme = 'dark' | 'light';
export type Accent = 'green' | 'blue' | 'purple' | 'orange' | 'red' | 'teal';

const THEME_KEY = 'rungine.theme';
const ACCENT_KEY = 'rungine.accent';

type AccentPalette = {
  dark: { base: string; hover: string; text: string };
  light: { base: string; hover: string; text: string };
};

export const ACCENTS: { id: Accent; label: string; swatch: string }[] = [
  { id: 'green', label: 'Green', swatch: '#4ade80' },
  { id: 'blue', label: 'Blue', swatch: '#60a5fa' },
  { id: 'purple', label: 'Purple', swatch: '#c084fc' },
  { id: 'orange', label: 'Orange', swatch: '#fb923c' },
  { id: 'red', label: 'Red', swatch: '#f87171' },
  { id: 'teal', label: 'Teal', swatch: '#2dd4bf' },
];

const PALETTES: Record<Accent, AccentPalette> = {
  green: {
    dark: { base: '#4ade80', hover: '#22c55e', text: '#0b1410' },
    light: { base: '#16a34a', hover: '#15803d', text: '#ffffff' },
  },
  blue: {
    dark: { base: '#60a5fa', hover: '#3b82f6', text: '#0a1426' },
    light: { base: '#2563eb', hover: '#1d4ed8', text: '#ffffff' },
  },
  purple: {
    dark: { base: '#c084fc', hover: '#a855f7', text: '#1a0d2b' },
    light: { base: '#9333ea', hover: '#7e22ce', text: '#ffffff' },
  },
  orange: {
    dark: { base: '#fb923c', hover: '#f97316', text: '#1f1207' },
    light: { base: '#ea580c', hover: '#c2410c', text: '#ffffff' },
  },
  red: {
    dark: { base: '#f87171', hover: '#ef4444', text: '#1c0808' },
    light: { base: '#dc2626', hover: '#b91c1c', text: '#ffffff' },
  },
  teal: {
    dark: { base: '#2dd4bf', hover: '#14b8a6', text: '#062019' },
    light: { base: '#0d9488', hover: '#0f766e', text: '#ffffff' },
  },
};

function readTheme(): Theme {
  const v = localStorage.getItem(THEME_KEY);
  return v === 'light' ? 'light' : 'dark';
}

function readAccent(): Accent {
  const v = localStorage.getItem(ACCENT_KEY);
  if (v && v in PALETTES) return v as Accent;
  return 'green';
}

export const theme = writable<Theme>(readTheme());
export const accent = writable<Accent>(readAccent());

function applyAccent(t: Theme, a: Accent) {
  const palette = PALETTES[a][t];
  const root = document.documentElement;
  root.style.setProperty('--accent', palette.base);
  root.style.setProperty('--accent-hover', palette.hover);
  root.style.setProperty('--accent-text', palette.text);
}

export function setTheme(t: Theme) {
  document.documentElement.dataset.theme = t;
  localStorage.setItem(THEME_KEY, t);
  theme.set(t);
  applyAccent(t, readAccent());
}

export function setAccent(a: Accent) {
  localStorage.setItem(ACCENT_KEY, a);
  accent.set(a);
  applyAccent(readTheme(), a);
}

export function initTheme() {
  setTheme(readTheme());
}
