import { writable } from 'svelte/store';

export type Route = 'tournaments' | 'analyze' | 'engines' | 'settings';

const ROUTES: Route[] = ['tournaments', 'analyze', 'engines', 'settings'];

function parse(): Route {
  const hash = window.location.hash.replace(/^#\/?/, '');
  const r = hash as Route;
  return ROUTES.includes(r) ? r : 'tournaments';
}

export const route = writable<Route>(parse());

window.addEventListener('hashchange', () => route.set(parse()));

export function navigate(r: Route) {
  if (parse() === r) return;
  window.location.hash = '/' + r;
}
