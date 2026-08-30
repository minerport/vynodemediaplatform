import { afterEach, expect, it, vi } from 'vitest';
import { api } from './api';

afterEach(() => vi.unstubAllGlobals());

it('keeps the Libraries page usable before any libraries exist', async () => {
  vi.stubGlobal('fetch', vi.fn().mockResolvedValue(
    new Response('{"libraries":null}', { status: 200 }),
  ));
  expect((await api.libraries()).libraries).toEqual([]);
});

it('keeps a new library usable before its first scan', async () => {
  vi.stubGlobal('fetch', vi.fn().mockResolvedValue(
    new Response('{"items":null}', { status: 200 }),
  ));
  expect((await api.items('empty-library')).items).toEqual([]);
});

it('keeps the Movies page list usable for an empty server catalog', async () => {
  vi.stubGlobal('fetch', vi.fn().mockResolvedValue(
    new Response('{"movies":null,"limit":100,"offset":0}', { status: 200 }),
  ));
  const result = await api.movies();
  expect(result.movies).toEqual([]);
  expect(result.movies.length).toBe(0);
});
